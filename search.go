package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var frontmatterPattern = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?`)

type frontmatter struct {
	Tags []string `yaml:"tags"`
}

// parseFrontmatter extracts YAML frontmatter (delimited by --- lines) from
// the start of a note, returning its tags and the remaining body
func parseFrontmatter(content string) ([]string, string) {
	m := frontmatterPattern.FindStringSubmatch(content)
	if m == nil {
		return nil, content
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
		return nil, content
	}

	body := content[len(m[0]):]
	return fm.Tags, body
}

// GetTags returns the tags declared in the given note's frontmatter
func (a *App) GetTags(name string) ([]string, error) {
	data, err := os.ReadFile(a.notePath(name))
	if err != nil {
		return nil, err
	}
	tags, _ := parseFrontmatter(string(data))
	if tags == nil {
		tags = []string{}
	}
	sort.Strings(tags)
	return tags, nil
}

// ListAllTags returns every unique tag used across the vault, sorted alphabetically
func (a *App) ListAllTags() ([]string, error) {
	seen := map[string]bool{}
	err := a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		tags, _ := parseFrontmatter(string(data))
		for _, t := range tags {
			seen[t] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags, nil
}

// TagCount holds a tag and how many notes use it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// GetTagCounts returns every tag in the vault with its note count, sorted by count desc then name asc.
func (a *App) GetTagCounts() ([]TagCount, error) {
	counts := map[string]int{}
	err := a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		tags, _ := parseFrontmatter(string(data))
		for _, t := range tags {
			counts[t]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]TagCount, 0, len(counts))
	for t, c := range counts {
		result = append(result, TagCount{Tag: t, Count: c})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Tag < result[j].Tag
	})
	return result, nil
}

// parseFrontmatterFields decodes all YAML frontmatter fields into a string map.
func parseFrontmatterFields(content string) map[string]string {
	fields := map[string]string{}
	m := frontmatterPattern.FindStringSubmatch(content)
	if m == nil {
		return fields
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(m[1]), &raw); err != nil {
		return fields
	}
	for k, v := range raw {
		switch val := v.(type) {
		case []interface{}:
			parts := make([]string, 0, len(val))
			for _, item := range val {
				parts = append(parts, fmt.Sprint(item))
			}
			fields[k] = strings.Join(parts, ", ")
		default:
			fields[k] = fmt.Sprint(val)
		}
	}
	return fields
}

// compareVals compares two strings numerically if possible, otherwise lexicographically.
func compareVals(a, b string) int {
	var fa, fb float64
	_, ea := fmt.Sscanf(a, "%f", &fa)
	_, eb := fmt.Sscanf(b, "%f", &fb)
	if ea == nil && eb == nil {
		if fa < fb {
			return -1
		}
		if fa > fb {
			return 1
		}
		return 0
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// evalCondition evaluates a single WHERE condition against a note's fields.
func evalCondition(cond string, fields map[string]string) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return true
	}
	lower := strings.ToLower(cond)

	// "contains" operator
	if idx := strings.Index(lower, " contains "); idx >= 0 {
		field := strings.TrimSpace(cond[:idx])
		value := strings.ToLower(strings.Trim(strings.TrimSpace(cond[idx+10:]), `"'`))
		return strings.Contains(strings.ToLower(fields[field]), value)
	}
	for _, op := range []string{"!=", ">=", "<=", "=", ">", "<"} {
		sep := " " + op + " "
		if idx := strings.Index(cond, sep); idx >= 0 {
			field := strings.TrimSpace(cond[:idx])
			value := strings.Trim(strings.TrimSpace(cond[idx+len(sep):]), `"'`)
			fv := fields[field]
			switch op {
			case "=":
				return strings.EqualFold(fv, value)
			case "!=":
				return !strings.EqualFold(fv, value)
			case ">":
				return compareVals(fv, value) > 0
			case "<":
				return compareVals(fv, value) < 0
			case ">=":
				return compareVals(fv, value) >= 0
			case "<=":
				return compareVals(fv, value) <= 0
			}
		}
	}
	return true
}

// DataviewResult is returned by QueryNotes.
type DataviewResult struct {
	Mode    string              `json:"mode"`
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
	Error   string              `json:"error,omitempty"`
}

// QueryNotes executes a Dataview-like query against all vault notes.
func (a *App) QueryNotes(query string) DataviewResult {
	if a.vaultPath == "" {
		return DataviewResult{Error: "no vault open"}
	}

	// --- parse query ---
	mode := "list"
	var columns []string
	var fromTags []string
	var fromPaths []string
	var conditions []string
	sortField := ""
	sortAsc := true
	limit := 200

	for _, rawLine := range strings.Split(strings.TrimSpace(query), "\n") {
		line := strings.TrimSpace(rawLine)
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "TABLE"):
			mode = "table"
			rest := strings.TrimSpace(line[5:])
			if rest != "" {
				for _, c := range strings.Split(rest, ",") {
					columns = append(columns, strings.TrimSpace(c))
				}
			}
		case up == "LIST":
			mode = "list"
		case strings.HasPrefix(up, "FROM "):
			for _, tok := range strings.Fields(line[5:]) {
				if strings.EqualFold(tok, "AND") || strings.EqualFold(tok, "OR") {
					continue
				}
				if strings.HasPrefix(tok, "#") {
					fromTags = append(fromTags, strings.TrimPrefix(tok, "#"))
				} else {
					fromPaths = append(fromPaths, strings.Trim(tok, `"'`))
				}
			}
		case strings.HasPrefix(up, "WHERE "):
			// Split multiple AND conditions on one line
			for _, part := range strings.Split(line[6:], " AND ") {
				if t := strings.TrimSpace(part); t != "" {
					conditions = append(conditions, t)
				}
			}
		case strings.HasPrefix(up, "SORT "):
			parts := strings.Fields(line[5:])
			if len(parts) > 0 {
				sortField = parts[0]
			}
			if len(parts) > 1 && strings.ToUpper(parts[1]) == "DESC" {
				sortAsc = false
			}
		case strings.HasPrefix(up, "LIMIT "):
			fmt.Sscanf(strings.TrimSpace(line[6:]), "%d", &limit)
		}
	}

	// --- walk vault ---
	var rows []map[string]string

	_ = filepath.WalkDir(a.vaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(a.vaultPath, path)
		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		tags, _ := parseFrontmatter(content)
		fields := parseFrontmatterFields(content)

		// built-in fields
		name := strings.TrimSuffix(filepath.Base(rel), ".md")
		fields["file"] = rel
		fields["file.name"] = name
		fields["file.path"] = rel
		if info, err := d.Info(); err == nil {
			fields["file.mtime"] = info.ModTime().Format("2006-01-02")
		}
		if len(tags) > 0 {
			fields["tags"] = strings.Join(tags, ", ")
		}

		// FROM tag filter (OR between tags)
		if len(fromTags) > 0 {
			matched := false
			for _, ft := range fromTags {
				for _, t := range tags {
					if strings.EqualFold(t, ft) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// FROM path filter
		if len(fromPaths) > 0 {
			matched := false
			for _, fp := range fromPaths {
				if strings.HasPrefix(rel, fp) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// WHERE conditions (all must pass)
		for _, cond := range conditions {
			if !evalCondition(cond, fields) {
				return nil
			}
		}

		// build row
		row := map[string]string{"file": rel, "file.name": name}
		if mode == "table" && len(columns) > 0 {
			for _, col := range columns {
				row[col] = fields[col]
			}
		} else {
			for k, v := range fields {
				row[k] = v
			}
		}
		rows = append(rows, row)
		return nil
	})

	// sort
	if sortField != "" {
		sort.SliceStable(rows, func(i, j int) bool {
			vi, vj := rows[i][sortField], rows[j][sortField]
			cmp := compareVals(vi, vj)
			if sortAsc {
				return cmp < 0
			}
			return cmp > 0
		})
	}

	// limit
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if rows == nil {
		rows = []map[string]string{}
	}

	cols := columns
	if len(cols) == 0 && mode == "table" {
		cols = []string{"file.name"}
	}

	return DataviewResult{Mode: mode, Columns: cols, Rows: rows}
}

// SearchByTag returns the paths of notes whose frontmatter contains the given tag
func (a *App) SearchByTag(tag string) ([]string, error) {
	names := []string{}
	err := a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		tags, _ := parseFrontmatter(string(data))
		for _, t := range tags {
			if t == tag {
				names = append(names, relPath)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// SearchNotes returns the paths of notes whose path or content contains query
type searchToken struct {
	kind  string // "", "tag", "file", "path", "line", or "section"
	value string
}

func parseSearchQuery(query string) []searchToken {
	var tokens []searchToken
	for _, raw := range strings.Fields(query) {
		kind, value := "", raw
		for _, p := range []string{"tag:", "file:", "path:", "line:", "section:"} {
			if strings.HasPrefix(strings.ToLower(raw), p) {
				kind = strings.TrimSuffix(p, ":")
				value = raw[len(p):]
				break
			}
		}
		value = strings.ToLower(value)
		if kind == "tag" {
			value = strings.TrimPrefix(value, "#")
		}
		if value == "" {
			continue
		}
		tokens = append(tokens, searchToken{kind, value})
	}
	return tokens
}

// splitSections breaks markdown content into chunks delimited by ATX headings
func splitSections(content string) []string {
	headingPattern := regexp.MustCompile(`(?m)^#{1,6}\s`)
	var sections []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if headingPattern.MatchString(line) && current.Len() > 0 {
			sections = append(sections, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}

// SearchNotes returns the paths of notes matching a space-separated query.
// Bare terms match the note's path or content. Terms may be prefixed with
// tag:, file:, path:, line:, or section: to scope the match; all terms must
// match (AND) for a note to be included.
// SearchHit is a single note result with matching snippets.
type SearchHit struct {
	Path     string   `json:"path"`
	Snippets []string `json:"snippets"`
}

// SearchWithSnippets performs case-insensitive full-text search and returns
// up to 3 matching line snippets per note.
func (a *App) SearchWithSnippets(query string) []SearchHit {
	if strings.TrimSpace(query) == "" {
		return []SearchHit{}
	}
	ql := strings.ToLower(query)
	var hits []SearchHit
	_ = a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		var snippets []string
		for _, line := range strings.Split(string(data), "\n") {
			lower := strings.ToLower(line)
			if !strings.Contains(lower, ql) {
				continue
			}
			s := strings.TrimSpace(line)
			if len(s) > 120 {
				idx := strings.Index(strings.ToLower(s), ql)
				start := idx - 40
				if start < 0 {
					start = 0
				}
				end := idx + len(query) + 60
				if end > len(s) {
					end = len(s)
				}
				s = "..." + s[start:end] + "..."
			}
			snippets = append(snippets, s)
			if len(snippets) >= 3 {
				break
			}
		}
		if len(snippets) > 0 {
			hits = append(hits, SearchHit{Path: relPath, Snippets: snippets})
		}
		return nil
	})
	return hits
}

func (a *App) SearchNotes(query string) ([]string, error) {
	tokens := parseSearchQuery(query)
	if len(tokens) == 0 {
		return a.ListNotes()
	}

	names := []string{}
	err := a.walkNotes(func(relPath string) error {
		data, err := os.ReadFile(filepath.Join(a.vaultPath, relPath+".md"))
		if err != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		lowerPath := strings.ToLower(relPath)
		base := lowerPath
		if i := strings.LastIndex(base, "/"); i != -1 {
			base = base[i+1:]
		}

		var tags []string
		var lines, sections []string
		tagsLoaded, linesLoaded, sectionsLoaded := false, false, false

		for _, t := range tokens {
			matched := false
			switch t.kind {
			case "tag":
				if !tagsLoaded {
					tags, _ = parseFrontmatter(string(data))
					tagsLoaded = true
				}
				for _, tg := range tags {
					if strings.Contains(strings.ToLower(tg), t.value) {
						matched = true
						break
					}
				}
			case "file":
				matched = strings.Contains(base, t.value)
			case "path":
				matched = strings.Contains(lowerPath, t.value)
			case "line":
				if !linesLoaded {
					lines = strings.Split(content, "\n")
					linesLoaded = true
				}
				for _, ln := range lines {
					if strings.Contains(ln, t.value) {
						matched = true
						break
					}
				}
			case "section":
				if !sectionsLoaded {
					sections = splitSections(content)
					sectionsLoaded = true
				}
				for _, sec := range sections {
					if strings.Contains(sec, t.value) {
						matched = true
						break
					}
				}
			default:
				matched = strings.Contains(lowerPath, t.value) || strings.Contains(content, t.value)
			}
			if !matched {
				return nil
			}
		}
		names = append(names, relPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}
