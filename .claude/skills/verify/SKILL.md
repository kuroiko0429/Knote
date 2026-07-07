---
name: verify
description: knote-devの変更をランタイム検証する手順。Wails GUIアプリだがブラウザ経由で実バックエンドごと駆動できる。
---

# knote-dev の検証手順

## 起動
```bash
cd /home/kuroiko/Documents/knote-dev && wails dev
```
バックグラウンドで起動し、http://localhost:34115 が200を返すまでポーリング（初回コンパイルで1分弱かかる）。
このURLはWailsのdevサーバで、ブラウザからでもGoバインディングがフルに使える。

## 駆動
- Playwright MCPプラグインはchromeチャンネル固定で動かない。`playwright-core` をscratchpadに入れて
  `~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome` を executablePath に指定する。
- vite devはHMRが効くので、.svelteを編集したらリロード不要で再テストできる。
- テストvault: `~/.config/knote/config.json` の vaultPath（現在 /home/kuroiko/Knote）。
  Marpノートは `marp: true` をfrontmatterに持つ .md。

## 罠
- `.svelte` のscript内に `<!--` を含むregexリテラルや文字列を書くと、本番ビルドで `undefined` に化けることがある
  （テンプレート文字列内のregexリテラルも対象）。検証時は `npx vite build` 後に
  `grep -c "undefined/g" dist/assets/*.js` で確認。`new RegExp('<'+'!--...')` の連結形は安全。
- 本番のWebViewはWebKitGTK。bind:clientWidthはマウント時1回しか更新されない → ResizeObserverアクションを使う。
- ydotoolでの実ウィンドウクリックは禁止（Claude Code自身のプロンプトからフォーカスを奪う）。
