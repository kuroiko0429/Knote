package main

import (
	"bytes"
	"context"

	"github.com/yuin/goldmark"
)

// App struct
type App struct {
	ctx context.Context
	md  goldmark.Markdown
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{md: goldmark.New()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// RenderMarkdown converts markdown source to HTML
func (a *App) RenderMarkdown(src string) string {
	var buf bytes.Buffer
	if err := a.md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return buf.String()
}
