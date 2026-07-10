package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

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

// BacklinkItem holds a note path and the lines that reference the target.
type BacklinkItem struct {
	Note     string   `json:"note"`
	Snippets []string `json:"snippets"`
}

// GetBacklinksWithContext returns backlinks with the surrounding context lines.
func (a *App) GetBacklinksWithContext(name string) ([]BacklinkItem, error) {
	pattern := regexp.MustCompile(`\[\[` + regexp.QuoteMeta(name) + `\]\]`)
	items := []BacklinkItem{}
	err := a.walkNotes(func(relPath string) error {
		if relPath == name {
			return nil
		}
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		snippets := []string{}
		for _, line := range strings.Split(string(data), "\n") {
			if pattern.MatchString(line) {
				s := strings.TrimSpace(line)
				if len(s) > 120 {
					s = s[:120] + "…"
				}
				snippets = append(snippets, s)
			}
		}
		if len(snippets) > 0 {
			items = append(items, BacklinkItem{Note: relPath, Snippets: snippets})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Note < items[j].Note })
	return items, nil
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
