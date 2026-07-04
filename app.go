package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	htmlstd "html"
	"github.com/creack/pty"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// App struct
type App struct {
	ctx               context.Context
	md                goldmark.Markdown
	vaultPath         string
	templatesFolder   string
	dailyNoteFolder   string
	dailyNoteTemplate string
	watcher           *fsnotify.Watcher
	ptmx              *os.File
	shellCmd          *exec.Cmd
}

type appConfig struct {
	VaultPath         string `json:"vaultPath"`
	TemplatesFolder   string `json:"templatesFolder"`
	DailyNoteFolder   string `json:"dailyNoteFolder"`
	DailyNoteTemplate string `json:"dailyNoteTemplate"`
	ActiveTheme       string `json:"activeTheme"`
}

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

// langPreWrapper injects class="language-<lang>" onto the <code> element so the
// frontend can detect the language for the run-button feature.
type langPreWrapper struct{ lang string }

func (p langPreWrapper) Start(code bool, styleAttr string) string {
	if code {
		return fmt.Sprintf(`<pre class="chroma"><code class="language-%s">`, htmlstd.EscapeString(p.lang))
	}
	return fmt.Sprintf(`<pre%s>`, styleAttr)
}

func (p langPreWrapper) End(code bool) string {
	if code {
		return `</code></pre>`
	}
	return `</pre>`
}

// NewApp creates a new App application struct
func NewApp() *App {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithStyle("onedark"),
				highlighting.WithCodeBlockOptions(func(ctx highlighting.CodeBlockContext) []chromahtml.Option {
					lang, ok := ctx.Language()
					if !ok {
						return nil
					}
					return []chromahtml.Option{chromahtml.WithPreWrapper(langPreWrapper{lang: string(lang)})}
				}),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	return &App{md: md}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg := a.loadConfig()
	if cfg.VaultPath != "" {
		a.vaultPath = cfg.VaultPath
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		a.vaultPath = filepath.Join(home, "Knote")
	}
	a.templatesFolder = cfg.TemplatesFolder
	if a.templatesFolder == "" {
		a.templatesFolder = "templates"
	}
	a.dailyNoteFolder = cfg.DailyNoteFolder
	if a.dailyNoteFolder == "" {
		a.dailyNoteFolder = "daily"
	}
	a.dailyNoteTemplate = cfg.DailyNoteTemplate

	os.MkdirAll(a.vaultPath, 0755)
	a.startWatcher()
	runtime.OnFileDrop(ctx, a.onFileDrop)
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

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	default:
		return false
	}
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

// RenderMarkdown converts markdown source to HTML. [[note]] wikilinks are
// rewritten into knote:// links so the frontend can intercept clicks on them.
// Any leading YAML frontmatter is stripped from the rendered output. Local
// image references are inlined as base64 data URIs so they render correctly
// inside the webview regardless of asset-serving restrictions.
func (a *App) RenderMarkdown(src string) string {
	_, body := parseFrontmatter(src)
	body = wikilinkPattern.ReplaceAllString(body, "[$1](<knote:$1>)")

	var buf bytes.Buffer
	if err := a.md.Convert([]byte(body), &buf); err != nil {
		return ""
	}
	return a.inlineImages(buf.String())
}

var (
	imgTagPattern  = regexp.MustCompile(`<img[^>]*>`)
	srcAttrPattern = regexp.MustCompile(`src="([^"]*)"`)
)

// inlineImages rewrites <img src="..."> tags whose src is a vault-relative
// path into base64 data URIs, leaving http(s)/data URLs untouched.
func (a *App) inlineImages(htmlStr string) string {
	return imgTagPattern.ReplaceAllStringFunc(htmlStr, func(tag string) string {
		m := srcAttrPattern.FindStringSubmatch(tag)
		if m == nil {
			return tag
		}
		src := m[1]
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "data:") {
			return tag
		}
		data, err := os.ReadFile(filepath.Join(a.vaultPath, src))
		if err != nil {
			return tag
		}
		mimeType := mime.TypeByExtension(filepath.Ext(src))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		newSrc := fmt.Sprintf(`src="data:%s;base64,%s"`, mimeType, encoded)
		return srcAttrPattern.ReplaceAllString(tag, newSrc)
	})
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

// PrepareRunFile writes code to a private temp file and returns the shell command to run it.
// This avoids glob expansion issues when sending multi-line code directly to the PTY.
func (a *App) PrepareRunFile(lang, code string) string {
	ext := ".sh"
	runner := "bash"
	switch strings.ToLower(lang) {
	case "python", "python3", "py":
		ext, runner = ".py", "python3"
	case "javascript", "js":
		ext, runner = ".js", "node"
	case "typescript", "ts":
		ext, runner = ".ts", "ts-node"
	case "ruby", "rb":
		ext, runner = ".rb", "ruby"
	case "go":
		ext, runner = ".go", "go run"
	}
	dir, err := os.MkdirTemp("", "knote_run_*")
	if err != nil {
		return ""
	}
	_ = os.Chmod(dir, 0o700)
	tmpFile := filepath.Join(dir, "run"+ext)
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return ""
	}
	_, _ = f.WriteString(code + "\n")
	f.Close()
	quoted := "'" + strings.ReplaceAll(tmpFile, "'", `'\''`) + "'"
	return runner + " " + quoted
}

func (a *App) exportHTML(notePath string) (string, error) {
	vaultAbs, err := filepath.Abs(a.vaultPath)
	if err != nil {
		return "", err
	}
	fullNote := filepath.Clean(filepath.Join(vaultAbs, notePath))
	rel, err := filepath.Rel(vaultAbs, fullNote)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid note path")
	}

	content, err := a.ReadNote(notePath)
	if err != nil {
		return "", err
	}
	title := filepath.Base(notePath)

	body := a.RenderMarkdown(content)
	// strip knote:// links — replace with plain text spans
	body = regexp.MustCompile(`href="<knote:([^"]+)>"`).ReplaceAllString(body, `href="#" data-knote="$1"`)

	html := `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>` + htmlstd.EscapeString(title) + `</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:16px;line-height:1.7;color:#222;max-width:800px;margin:0 auto;padding:2rem}
h1,h2,h3,h4,h5,h6{font-weight:700;margin:1.5em 0 0.5em;line-height:1.3}
h1{font-size:2em;border-bottom:2px solid #eee;padding-bottom:0.3em}
h2{font-size:1.5em;border-bottom:1px solid #eee;padding-bottom:0.2em}
p{margin:0.8em 0}
a{color:#0969da;text-decoration:none}
a:hover{text-decoration:underline}
code{font-family:'SFMono-Regular',Consolas,'Liberation Mono',monospace;font-size:0.875em;background:#f6f8fa;padding:0.2em 0.4em;border-radius:3px}
pre{background:#282c34;color:#abb2bf;padding:1rem;border-radius:6px;overflow-x:auto;margin:1em 0}
pre code{background:none;padding:0;font-size:0.875em}
blockquote{border-left:4px solid #d0d7de;padding:0 1em;color:#656d76;margin:1em 0}
ul,ol{padding-left:2em;margin:0.8em 0}
li{margin:0.3em 0}
table{border-collapse:collapse;width:100%;margin:1em 0}
th,td{border:1px solid #d0d7de;padding:0.5em 0.75em;text-align:left}
th{background:#f6f8fa;font-weight:600}
tr:nth-child(even){background:#f6f8fa}
img{max-width:100%;height:auto}
hr{border:none;border-top:1px solid #eee;margin:1.5em 0}
.chroma{background:#282c34;border-radius:6px;overflow-x:auto}
</style>
</head>
<body>
` + body + `
</body>
</html>`

	base := filepath.Base(notePath)
	noteDir := filepath.Dir(fullNote)
	outPath := filepath.Join(noteDir, strings.TrimSuffix(base, filepath.Ext(base))+".html")
	if err := os.WriteFile(outPath, []byte(html), 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

func (a *App) ExportHTML(notePath string) (string, error) {
	return a.exportHTML(notePath)
}

func (a *App) ExportPDF(notePath string) (string, error) {
	htmlPath, err := a.exportHTML(notePath)
	if err != nil {
		return "", err
	}
	defer os.Remove(htmlPath)

	vaultAbs, _ := filepath.Abs(a.vaultPath)
	fullNote := filepath.Clean(filepath.Join(vaultAbs, notePath))
	noteDir := filepath.Dir(fullNote)
	base := strings.TrimSuffix(filepath.Base(notePath), filepath.Ext(notePath))
	outPath := filepath.Join(noteDir, base+".pdf")

	browsers := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"}
	var cmd *exec.Cmd
	for _, b := range browsers {
		if _, err := exec.LookPath(b); err == nil {
			cmd = exec.Command(b,
				"--headless",
				"--disable-gpu",
				"--no-sandbox",
				"--print-to-pdf="+outPath,
				"--print-to-pdf-no-header",
				"file://"+htmlPath,
			)
			break
		}
	}
	if cmd == nil {
		return "", fmt.Errorf("chromium が見つかりません。chromium または google-chrome をインストールしてください")
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("PDF 生成失敗: %w\n%s", err, out)
	}
	return outPath, nil
}

func (a *App) themeDir() string {
	return filepath.Join(a.vaultPath, ".knote", "theme")
}

func (a *App) ListThemes() []string {
	entries, err := os.ReadDir(a.themeDir())
	if err != nil {
		return []string{}
	}
	var names []string
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
