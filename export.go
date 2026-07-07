package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	htmlstd "html"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// imageWikilinkPattern is used by RenderMarkdown to detect ![[image]] embeds
var imageWikilinkPattern = regexp.MustCompile(`!\[\[([^\]\[]+)\]\]`)

// RenderMarkdown converts markdown source to HTML. [[note]] wikilinks are
// rewritten into knote:// links so the frontend can intercept clicks on them.
// Any leading YAML frontmatter is stripped from the rendered output. Local
// image references are inlined as base64 data URIs so they render correctly
// inside the webview regardless of asset-serving restrictions.
func (a *App) RenderMarkdown(src string) string {
	_, body := parseFrontmatter(src)
	body = imageWikilinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		inner := imageWikilinkPattern.FindStringSubmatch(match)[1]
		resolved := a.resolveImagePath(inner)
		if resolved == "" {
			return match
		}
		return fmt.Sprintf("![%s](%s)", filepath.Base(inner), resolved)
	})
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
