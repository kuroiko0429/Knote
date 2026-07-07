# knote リファクタ設計: 挙動保存の分割

日付: 2026-07-07
ステータス: 承認済み

## ゴール

機能・見た目・挙動を一切変えずに、`frontend/src/App.svelte`（4914行）と `app.go`（1724行）を機能単位に分割する。分割中に見つかった死にコード（未使用関数・未使用CSSセレクタ）は削除する。UI改善・機能追加はこのリファクタ完了後の別フェーズで行う。

## 非ゴール

- Svelte 5 / runes への移行
- 機能の追加・変更・削除（死にコード削除を除く）
- スタイルの変更（CSSは移動のみ）

## 現状

- `App.svelte`: script部 1–2140行、マークアップ 2141–2834行、CSS 2835–4914行
- `app.go`: 全バックエンドメソッドが1ファイルに同居
- ビルド警告: 未使用CSSセレクタ `.rename-input`、a11y警告多数（a11yは今回スコープ外）

## フロントエンド分割

CSSは対応するコンポーネントに同伴移動する。子コンポーネントへは props / `createEventDispatcher` が基本。store は境界をまたぐ横断的状態のみ。

| 新ファイル | 中身 | 目安 |
|---|---|---|
| `src/MarpPreview.svelte` | Marpレンダリング・サムネイルストリップ・プレゼンモード一式 | ~700行 |
| `src/Settings.svelte` | 設定モーダル（フォント・テーマ・フォルダ設定） | ~400行 |
| `src/QuickSwitcher.svelte` | コマンドパレット / クイックスイッチャー | ~300行 |
| `src/Sidebar.svelte` | ファイルツリー・検索・タグパネル | ~600行 |
| `src/lib/editor.ts` | CodeMirror拡張・Vim設定・wikilink補完（UIなし純ロジック） | ~500行 |
| `src/lib/markdown.ts` | KaTeX / Mermaid / Dataview の後処理 | ~200行 |
| `src/lib/stores.ts` | トースト・現在ノート・フォント設定など横断状態のみ | 小 |

`App.svelte` に残るもの: レイアウト、タブ管理、ペイン分割ドラッグ、子コンポーネントのオーケストレーション。目標 1500行以下。

## バックエンド分割

`package main` のまま、純粋なファイル移動。`app.go` には `App` 構造体・`startup`・設定ロード周りの骨格だけ残す。

| 新ファイル | 中身 |
|---|---|
| `config.go` | configPath / loadConfig / saveConfig / 各種 Get/Set |
| `notes.go` | ノートCRUD・rename・wikilink更新・walkNotes |
| `search.go` | SearchNotes / SearchWithSnippets / タグ / Dataview (QueryNotes) |
| `links.go` | GetBacklinks(WithContext) / GetGraph |
| `marp.go` | Marpテーマ管理 |
| `export.go` | ExportHTML / ExportPDF / inlineImages |
| `media.go` | SaveImage / SelectImage / resolveImagePath / uniqueAttachmentPath |
| `snippets.go` | GetSnippets / SaveSnippets |
| `terminal.go` | PTY関連 / PrepareRunFile |
| `watcher.go` | startWatcher / onFileDrop |

## 進め方

1コンポーネント（または1バックエンドファイル群）切り出すごとに1コミット。各コミットで:

1. `npx vite build` が警告悪化なしで通る（`grep -c "undefined/g" dist/assets/*.js` が0）
2. `wails dev` + Playwright（`.claude/skills/verify/SKILL.md` の手順）で該当機能の回帰確認

死にコードは発見時に削除し、コミットメッセージに明記する。

## リスクと対策

- **`<!--` コンパイル化け**: script内に `<!--` を含むコードを移動する際は連結形を維持。ビルド後にgrepで検出
- **WebKitGTK差**: `bind:clientWidth` は使わず ResizeObserver アクションを維持
- **状態の暗黙結合**: App.svelte内の変数が複数機能から参照されている場合、切り出し時に依存を明示化（props化）し、雑にstoreへ逃がさない
