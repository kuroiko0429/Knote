package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func (a *App) GetMarpTheme() string { return a.marpTheme }

func (a *App) SetMarpTheme(theme string) error {
	a.marpTheme = theme
	return a.saveConfig()
}

type MarpCustomTheme struct {
	Name string `json:"name"`
	CSS  string `json:"css"`
}

func (a *App) GetMarpThemesDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "knote", "marp-themes")
	if err := os.MkdirAll(p, 0755); err != nil {
		return "", err
	}
	return p, nil
}

func (a *App) ListMarpCustomThemes() ([]MarpCustomTheme, error) {
	dir, err := a.GetMarpThemesDir()
	if err != nil {
		return []MarpCustomTheme{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []MarpCustomTheme{}, nil
	}
	themes := []MarpCustomTheme{}
	nameRe := regexp.MustCompile(`@theme\s+(\S+)`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".css") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".css")
		if m := nameRe.FindSubmatch(data); m != nil {
			name = string(m[1])
		}
		themes = append(themes, MarpCustomTheme{Name: name, CSS: string(data)})
	}
	return themes, nil
}
