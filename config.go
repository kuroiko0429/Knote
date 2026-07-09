package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type appConfig struct {
	VaultPath         string `json:"vaultPath"`
	TemplatesFolder   string `json:"templatesFolder"`
	DailyNoteFolder   string `json:"dailyNoteFolder"`
	DailyNoteTemplate string `json:"dailyNoteTemplate"`
	ActiveTheme       string `json:"activeTheme"`
	FontFamily        string `json:"fontFamily"`
	FontSize          int    `json:"fontSize"`
	PreviewFontFamily string `json:"previewFontFamily"`
	PreviewFontSize   int    `json:"previewFontSize"`
	MarpTheme         string `json:"marpTheme"`
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
	data, err := json.Marshal(appConfig{
		VaultPath:         a.vaultPath,
		TemplatesFolder:   a.templatesFolder,
		DailyNoteFolder:   a.dailyNoteFolder,
		DailyNoteTemplate: a.dailyNoteTemplate,
		FontFamily:        a.fontFamily,
		FontSize:          a.fontSize,
		PreviewFontFamily: a.previewFontFamily,
		PreviewFontSize:   a.previewFontSize,
		MarpTheme:         a.marpTheme,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetVaultPath returns the currently active vault directory
func (a *App) GetVaultPath() string {
	return a.vaultPath
}

// GetTemplatesFolder returns the vault-relative folder used for templates
func (a *App) GetTemplatesFolder() string {
	return a.templatesFolder
}

// SetTemplatesFolder updates the vault-relative folder used for templates
func (a *App) SetTemplatesFolder(path string) error {
	a.templatesFolder = path
	return a.saveConfig()
}

// GetDailyNoteFolder returns the vault-relative folder used for daily notes
func (a *App) GetDailyNoteFolder() string {
	return a.dailyNoteFolder
}

// SetDailyNoteFolder updates the vault-relative folder used for daily notes
func (a *App) SetDailyNoteFolder(path string) error {
	a.dailyNoteFolder = path
	return a.saveConfig()
}

// GetDailyNoteTemplate returns the template name used for new daily notes
// (empty means no template)
func (a *App) GetDailyNoteTemplate() string {
	return a.dailyNoteTemplate
}

// SetDailyNoteTemplate updates the template used for new daily notes
func (a *App) SetDailyNoteTemplate(name string) error {
	a.dailyNoteTemplate = name
	return a.saveConfig()
}

func (a *App) GetFontFamily() string { return a.fontFamily }
func (a *App) GetFontSize() int      { return a.fontSize }

func (a *App) SetFontFamily(family string) error {
	a.fontFamily = family
	return a.saveConfig()
}

func (a *App) SetFontSize(size int) error {
	a.fontSize = size
	return a.saveConfig()
}

func (a *App) GetPreviewFontFamily() string { return a.previewFontFamily }
func (a *App) GetPreviewFontSize() int      { return a.previewFontSize }

func (a *App) SetPreviewFontFamily(family string) error {
	a.previewFontFamily = family
	return a.saveConfig()
}

func (a *App) SetPreviewFontSize(size int) error {
	a.previewFontSize = size
	return a.saveConfig()
}

// ListTemplates returns the names of templates in the templates folder
func (a *App) ListTemplates() ([]string, error) {
	dir := filepath.Join(a.vaultPath, a.templatesFolder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	names := []string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
}

// GetTemplateContent returns the raw content of the given template
func (a *App) GetTemplateContent(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(a.vaultPath, a.templatesFolder, name+".md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func (a *App) themeDir() string {
	return filepath.Join(a.vaultPath, ".knote", "theme")
}

func (a *App) ListThemes() []string {
	entries, err := os.ReadDir(a.themeDir())
	if err != nil {
		return []string{}
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") {
			names = append(names, strings.TrimSuffix(e.Name(), ".css"))
		}
	}
	return names
}

func (a *App) LoadTheme(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	safe := filepath.Clean(name + ".css")
	if strings.Contains(safe, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid theme name")
	}
	data, err := os.ReadFile(filepath.Join(a.themeDir(), safe))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) GetActiveTheme() string {
	return a.loadConfig().ActiveTheme
}

func (a *App) SetActiveTheme(name string) error {
	cfg := a.loadConfig()
	cfg.ActiveTheme = name
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	p, err := a.configPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (a *App) GetThemeDir() string {
	return a.themeDir()
}
