package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
)

// App struct
type App struct {
	ctx       context.Context
	md        goldmark.Markdown
	vaultPath string
}

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{md: goldmark.New()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	a.vaultPath = filepath.Join(home, "Knote")
	os.MkdirAll(a.vaultPath, 0755)
}

// RenderMarkdown converts markdown source to HTML. [[note]] wikilinks are
// rewritten into knote:// links so the frontend can intercept clicks on them.
func (a *App) RenderMarkdown(src string) string {
	src = wikilinkPattern.ReplaceAllString(src, "[$1](<knote:$1>)")

	var buf bytes.Buffer
	if err := a.md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return buf.String()
}

func (a *App) notePath(name string) string {
	return filepath.Join(a.vaultPath, name+".md")
}

// ListNotes returns the names of all notes in the vault, sorted alphabetically
func (a *App) ListNotes() ([]string, error) {
	entries, err := os.ReadDir(a.vaultPath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

// ReadNote returns the content of the given note
func (a *App) ReadNote(name string) (string, error) {
	data, err := os.ReadFile(a.notePath(name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveNote writes content to the given note, creating it if necessary
func (a *App) SaveNote(name string, content string) error {
	return os.WriteFile(a.notePath(name), []byte(content), 0644)
}

// CreateNote creates a new empty note. Returns an error if it already exists
func (a *App) CreateNote(name string) error {
	path := a.notePath(name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("note %q already exists", name)
	}
	return os.WriteFile(path, []byte(""), 0644)
}

// DeleteNote removes a note from the vault
func (a *App) DeleteNote(name string) error {
	return os.Remove(a.notePath(name))
}

// RenameNote renames a note. Returns an error if the new name is already taken
func (a *App) RenameNote(oldName string, newName string) error {
	newPath := a.notePath(newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("note %q already exists", newName)
	}
	return os.Rename(a.notePath(oldName), newPath)
}
