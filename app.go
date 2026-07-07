package main

import (
	"context"
	"fmt"
	htmlstd "html"
	"os"
	"os/exec"
	"path/filepath"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// App struct
type App struct {
	ctx               context.Context
	md                goldmark.Markdown
	vaultPath         string
	templatesFolder   string
	dailyNoteFolder   string
	dailyNoteTemplate string
	fontFamily        string
	fontSize          int
	previewFontFamily string
	previewFontSize   int
	marpTheme         string
	watcher           *fsnotify.Watcher
	ptmx              *os.File
	shellCmd          *exec.Cmd
}

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
	a.fontFamily = cfg.FontFamily
	a.fontSize = cfg.FontSize
	a.previewFontFamily = cfg.PreviewFontFamily
	a.previewFontSize = cfg.PreviewFontSize
	a.marpTheme = cfg.MarpTheme
	if a.marpTheme == "" {
		a.marpTheme = "default"
	}

	os.MkdirAll(a.vaultPath, 0755)
	a.startWatcher()
	runtime.OnFileDrop(ctx, a.onFileDrop)
}
