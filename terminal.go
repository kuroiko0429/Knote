package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
