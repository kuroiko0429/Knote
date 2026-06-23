package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

// App struct
type App struct {
	ctx       context.Context
	md        goldmark.Markdown
	vaultPath string
	watcher   *fsnotify.Watcher
	ptmx      *os.File
	shellCmd  *exec.Cmd
}

type appConfig struct {
	VaultPath string `json:"vaultPath"`
}

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

// NewApp creates a new App application struct
func NewApp() *App {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithStyle("onedark"),
			),
		),
	)
	return &App{md: md}
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
	a.startWatcher()
}

// startWatcher watches the vault directory (recursively) for external
// changes and notifies the frontend via a "vault:changed" event so it can
// refresh the note list.
func (a *App) startWatcher() {
	if a.watcher != nil {
		a.watcher.Close()
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	a.watcher = w

	filepath.WalkDir(a.vaultPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			w.Add(path)
		}
		return nil
	})

	go func() {
		var debounce *time.Timer
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						w.Add(event.Name)
					}
				}
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(300*time.Millisecond, func() {
					runtime.EventsEmit(a.ctx, "vault:changed")
				})
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
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
	a.startWatcher()
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

// walkNotes calls fn for every .md file in the vault, recursing into
// subfolders. relPath is the note's identity: its path relative to the
// vault root, "/"-separated, without the .md extension.
func (a *App) walkNotes(fn func(relPath string) error) error {
	return filepath.WalkDir(a.vaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(a.vaultPath, path)
		if err != nil {
			return err
		}
		return fn(filepath.ToSlash(strings.TrimSuffix(rel, ".md")))
	})
}

// ListNotes returns the paths of all notes in the vault, sorted alphabetically
func (a *App) ListNotes() ([]string, error) {
	names := []string{}
	if err := a.walkNotes(func(relPath string) error {
		names = append(names, relPath)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// ListFolders returns the paths of all folders in the vault, sorted alphabetically
func (a *App) ListFolders() ([]string, error) {
	folders := []string{}
	err := filepath.WalkDir(a.vaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == a.vaultPath || !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(a.vaultPath, path)
		if err != nil {
			return err
		}
		folders = append(folders, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(folders)
	return folders, nil
}

// CreateFolder creates a new folder. Returns an error if it already exists
func (a *App) CreateFolder(relPath string) error {
	path := filepath.Join(a.vaultPath, relPath)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("folder %q already exists", relPath)
	}
	return os.MkdirAll(path, 0755)
}

// RenameFolder renames a folder. Returns an error if the new path is already taken
func (a *App) RenameFolder(oldRelPath string, newRelPath string) error {
	newPath := filepath.Join(a.vaultPath, newRelPath)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("folder %q already exists", newRelPath)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}
	return os.Rename(filepath.Join(a.vaultPath, oldRelPath), newPath)
}

// SearchNotes returns the paths of notes whose path or content contains query
func (a *App) SearchNotes(query string) ([]string, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return a.ListNotes()
	}

	names := []string{}
	err := a.walkNotes(func(relPath string) error {
		if strings.Contains(strings.ToLower(relPath), q) {
			names = append(names, relPath)
			return nil
		}
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		if strings.Contains(strings.ToLower(string(data)), q) {
			names = append(names, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// GetBacklinks returns the paths of notes that contain a [[wikilink]] to the given note
func (a *App) GetBacklinks(name string) ([]string, error) {
	pattern := regexp.MustCompile(`\[\[` + regexp.QuoteMeta(name) + `\]\]`)
	names := []string{}
	err := a.walkNotes(func(relPath string) error {
		if relPath == name {
			return nil
		}
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		if pattern.Match(data) {
			names = append(names, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// GraphEdge is a single [[wikilink]] reference from one note to another
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// GraphData is the full set of notes and the wikilinks between them
type GraphData struct {
	Nodes []string    `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetGraph returns every note and every [[wikilink]] reference between them
func (a *App) GetGraph() (GraphData, error) {
	nodes, err := a.ListNotes()
	if err != nil {
		return GraphData{}, err
	}

	edges := []GraphEdge{}
	err = a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		for _, m := range wikilinkPattern.FindAllStringSubmatch(string(data), -1) {
			edges = append(edges, GraphEdge{Source: relPath, Target: m[1]})
		}
		return nil
	})
	if err != nil {
		return GraphData{}, err
	}

	return GraphData{Nodes: nodes, Edges: edges}, nil
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
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
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}
	if err := os.Rename(a.notePath(oldName), newPath); err != nil {
		return err
	}
	return a.updateWikilinks(oldName, newName)
}

// updateWikilinks rewrites every [[oldName]] occurrence across the vault to [[newName]]
func (a *App) updateWikilinks(oldName string, newName string) error {
	pattern := regexp.MustCompile(`\[\[` + regexp.QuoteMeta(oldName) + `\]\]`)
	replacement := "[[" + newName + "]]"

	return a.walkNotes(func(relPath string) error {
		path := filepath.Join(a.vaultPath, relPath+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !pattern.Match(data) {
			return nil
		}
		return os.WriteFile(path, pattern.ReplaceAll(data, []byte(replacement)), 0644)
	})
}

// StartTerminal spawns the user's shell attached to a pty, rooted at the
// vault directory. If a terminal session is already running, this is a
// no-op. Output is streamed to the frontend via "terminal:data" events.
func (a *App) StartTerminal() error {
	if a.ptmx != nil {
		return nil
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Dir = a.vaultPath
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	a.ptmx = ptmx
	a.shellCmd = cmd

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				runtime.EventsEmit(a.ctx, "terminal:data", string(buf[:n]))
			}
			if err != nil {
				a.ptmx = nil
				runtime.EventsEmit(a.ctx, "terminal:closed")
				return
			}
		}
	}()

	return nil
}

// WriteTerminal sends keyboard input to the running terminal session
func (a *App) WriteTerminal(data string) {
	if a.ptmx != nil {
		a.ptmx.Write([]byte(data))
	}
}

// ResizeTerminal resizes the pty to match the frontend's terminal dimensions
func (a *App) ResizeTerminal(cols int, rows int) {
	if a.ptmx != nil {
		pty.Setsize(a.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	}
}
