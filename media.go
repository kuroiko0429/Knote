package main

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	default:
		return false
	}
}

func (a *App) resolveImagePath(name string) string {
	if isImageExt(strings.ToLower(filepath.Ext(name))) {
		direct := filepath.Join(a.vaultPath, filepath.FromSlash(name))
		if _, err := os.Stat(direct); err == nil {
			return filepath.ToSlash(name)
		}
		var found string
		_ = filepath.WalkDir(a.vaultPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if filepath.Base(path) == filepath.Base(name) && isImageExt(strings.ToLower(filepath.Ext(path))) {
				rel, _ := filepath.Rel(a.vaultPath, path)
				found = filepath.ToSlash(rel)
				return fs.SkipAll
			}
			return nil
		})
		return found
	}
	return ""
}

// uniqueAttachmentPath returns a collision-free absolute path (and its
// vault-relative form) for filename inside the vault's attachments folder
func (a *App) uniqueAttachmentPath(filename string) (string, string) {
	dir := filepath.Join(a.vaultPath, "attachments")
	os.MkdirAll(dir, 0755)

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if base == "" {
		base = "image"
	}

	name := base + ext
	path := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s-%d%s", base, i, ext)
		path = filepath.Join(dir, name)
	}
	return path, "attachments/" + name
}

// SaveImage writes base64-encoded image data into the vault's attachments
// folder, returning the vault-relative path to reference it from markdown
func (a *App) SaveImage(filename string, base64Data string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}
	path, relPath := a.uniqueAttachmentPath(filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return relPath, nil
}

// SelectImage opens a native file picker for the user to choose an image,
// copies it into the vault's attachments folder, and returns the
// vault-relative path. Returns an empty string if the dialog was cancelled.
func (a *App) SelectImage() (string, error) {
	picked, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "画像を選択",
		Filters: []runtime.FileFilter{
			{DisplayName: "Images (*.png, *.jpg, *.jpeg, *.gif, *.webp, *.bmp, *.svg)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp;*.svg"},
		},
	})
	if err != nil || picked == "" {
		return "", err
	}

	data, err := os.ReadFile(picked)
	if err != nil {
		return "", err
	}

	path, relPath := a.uniqueAttachmentPath(filepath.Base(picked))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return relPath, nil
}

func (a *App) OpenPath(path string) error {
	return exec.Command("xdg-open", path).Start()
}
