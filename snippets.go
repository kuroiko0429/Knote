package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Snippet is a user-defined text snippet with a trigger keyword.
type Snippet struct {
	Trigger string `json:"trigger"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// GetSnippets reads snippets from .knote/snippets.json.
func (a *App) GetSnippets() ([]Snippet, error) {
	if a.vaultPath == "" {
		return []Snippet{}, nil
	}
	path := filepath.Join(a.vaultPath, ".knote", "snippets.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Snippet{}, nil
	}
	if err != nil {
		return nil, err
	}
	var snippets []Snippet
	if err := json.Unmarshal(data, &snippets); err != nil {
		return nil, err
	}
	return snippets, nil
}

// SaveSnippets writes snippets to .knote/snippets.json.
func (a *App) SaveSnippets(snippets []Snippet) error {
	dir := filepath.Join(a.vaultPath, ".knote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snippets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "snippets.json"), data, 0644)
}
