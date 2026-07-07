package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
		if d.IsDir() {
			if path != a.vaultPath && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !strings.HasSuffix(d.Name(), ".md") {
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
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == a.vaultPath {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
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
