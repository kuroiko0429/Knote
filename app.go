package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yuin/goldmark"
)

// App struct
type App struct {
	ctx       context.Context
	md        goldmark.Markdown
	vaultPath string
}

type appConfig struct {
	VaultPath string `json:"vaultPath"`
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

	if saved := a.loadConfig().VaultPath; saved != "" {
		a.vaultPath = saved
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		a.vaultPath = filepath.Join(home, "Knote")
	}
	os.MkdirAll(a.vaultPath, 0755)
}

func (a *App) configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "knote", "config.json"), nil
}

func (a *App) loadConfig() appConfig {
	var c appConfig
	path, err := a.configPath()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	json.Unmarshal(data, &c)
	return c
}

func (a *App) saveConfig() error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(appConfig{VaultPath: a.vaultPath})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetVaultPath returns the currently active vault directory
func (a *App) GetVaultPath() string {
	return a.vaultPath
}

// SelectVault opens a folder picker and switches the vault to the chosen
// directory. Returns the active vault path (unchanged if the dialog was
// cancelled).
func (a *App) SelectVault() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Knoteの保存先フォルダを選択",
		DefaultDirectory: a.vaultPath,
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return a.vaultPath, nil
	}

	a.vaultPath = dir
	if err := os.MkdirAll(a.vaultPath, 0755); err != nil {
		return "", err
	}
	if err := a.saveConfig(); err != nil {
		return "", err
	}
	return a.vaultPath, nil
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

// SearchNotes returns the names of notes whose title or content contains query
func (a *App) SearchNotes(query string) ([]string, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return a.ListNotes()
	}

	entries, err := os.ReadDir(a.vaultPath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if strings.Contains(strings.ToLower(name), q) {
			names = append(names, name)
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.vaultPath, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(data)), q) {
			names = append(names, name)
		}
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

// RenameNote renames a note and rewrites any [[wikilinks]] to it in other
// notes. Returns an error if the new name is already taken
func (a *App) RenameNote(oldName string, newName string) error {
	newPath := a.notePath(newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("note %q already exists", newName)
	}
	if err := os.Rename(a.notePath(oldName), newPath); err != nil {
		return err
	}
	return a.updateWikilinks(oldName, newName)
}

// updateWikilinks rewrites every [[oldName]] occurrence across the vault to [[newName]]
func (a *App) updateWikilinks(oldName string, newName string) error {
	entries, err := os.ReadDir(a.vaultPath)
	if err != nil {
		return err
	}

	pattern := regexp.MustCompile(`\[\[` + regexp.QuoteMeta(oldName) + `\]\]`)
	replacement := "[[" + newName + "]]"

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(a.vaultPath, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !pattern.Match(data) {
			continue
		}
		if err := os.WriteFile(path, pattern.ReplaceAll(data, []byte(replacement)), 0644); err != nil {
			return err
		}
	}
	return nil
}
