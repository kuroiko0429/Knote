package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
		if err != nil || !d.IsDir() {
			return nil
		}
		if path != a.vaultPath && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		w.Add(path)
		return nil
	})

	go func() {
		var listDebounce *time.Timer
		noteDebounces := map[string]*time.Timer{}
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
				if listDebounce != nil {
					listDebounce.Stop()
				}
				listDebounce = time.AfterFunc(300*time.Millisecond, func() {
					runtime.EventsEmit(a.ctx, "vault:changed")
				})
				if strings.HasSuffix(event.Name, ".md") {
					path := event.Name
					if t, ok := noteDebounces[path]; ok {
						t.Stop()
					}
					noteDebounces[path] = time.AfterFunc(200*time.Millisecond, func() {
						rel, err := filepath.Rel(a.vaultPath, path)
						if err != nil {
							return
						}
						noteName := strings.TrimSuffix(rel, ".md")
						runtime.EventsEmit(a.ctx, "vault:note-changed", noteName)
					})
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

// onFileDrop handles files dropped onto the window via Wails' native OS-level
// drag and drop (more reliable than the webview's own HTML5 DnD on Linux).
// Image files are copied into the vault's attachments folder; the frontend
// is notified via an "image:dropped" event so it can insert a markdown
// reference at the drop position.
func (a *App) onFileDrop(x int, y int, paths []string) {
	for _, p := range paths {
		if !isImageExt(strings.ToLower(filepath.Ext(p))) {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		path, relPath := a.uniqueAttachmentPath(filepath.Base(p))
		if err := os.WriteFile(path, data, 0644); err != nil {
			continue
		}
		runtime.EventsEmit(a.ctx, "image:dropped", x, y, relPath)
	}
}
