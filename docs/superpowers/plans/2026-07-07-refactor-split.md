# knote 分割リファクタ 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 挙動を一切変えずに App.svelte（4914行）と app.go（1724行）を機能単位のファイルに分割し、死にコードを削除する。

**Architecture:** 純粋な「移動+パラメータ化」。App.svelte内のクロージャ変数参照は、切り出し先では引数/props/コールバックに置き換える。バックエンドは `package main` のままファイル移動のみ。各タスク末尾でビルド+実アプリ回帰確認+コミット。

**Tech Stack:** Svelte 3 (TS) / Wails v2 / Go / CodeMirror 6 / Marp Core / playwright-core（回帰確認用devDependency）

## Global Constraints

- 機能・見た目・挙動の変更禁止（死にコード削除のみ例外、削除はコミットメッセージに明記）
- Svelte 5移行しない。store導入は横断的状態（トースト・フォント設定）のみ
- `.svelte` のscript内に `<!--` を含むコードを書かない（コンパイルで `undefined` に化ける）。既存の `'<'+'!--'` 連結形を維持
- `bind:clientWidth` 禁止（WebKitGTKで動かない）。ResizeObserverアクションを維持
- 各コミット前に: `cd frontend && npx vite build` が通り、`grep -hc "undefined/g" dist/assets/*.js | paste -sd+ | bc` が 0
- コミットメッセージは既存スタイル（`refactor: ...` 日本語）+ `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`
- 行番号は commit d749eff 時点の参考値。**シンボル名で特定すること**（行はズレる）

## 移動の原則（全タスク共通）

切り出す関数の本体は変更しない。ただし App.svelte のスコープ変数を参照している場合、その参照を引数化する。**移動前に必ず本体を読み、App スコープの識別子を列挙してから移す**こと。本計画の「依存」欄は期待リストであり、実物とズレたら実物に従う。

回帰確認の起動手順は `.claude/skills/verify/SKILL.md` の通り:
```bash
cd /home/kuroiko/Documents/knote-dev && wails dev   # バックグラウンド
# http://localhost:34115 が 200 を返すまでポーリング（初回~1分）
node frontend/e2e/smoke.js                           # Task 2 で作成
```

---

### Task 1: バックエンド app.go の分割

**Files:**
- Create: `config.go`, `notes.go`, `search.go`, `links.go`, `marp.go`, `export.go`, `media.go`, `snippets.go`, `terminal.go`, `watcher.go`
- Modify: `app.go`（移動元。App構造体・型定義・startup・NewAppだけ残す）

**Interfaces:**
- Consumes: なし
- Produces: 変更なし（同一パッケージ内の純粋なファイル移動。全メソッドのシグネチャ不変）

- [ ] **Step 1: 移動マップ通りに関数を移す**

`package main` 宣言を各新ファイル先頭に付け、importは`goimports`で整える。移動マップ（app.goの行はd749eff時点）:

| 移動先 | 移す関数（レシーバ `(a *App)` 含む） |
|---|---|
| `config.go` | configPath, loadConfig, saveConfig, GetVaultPath, GetTemplatesFolder, SetTemplatesFolder, GetDailyNoteFolder, SetDailyNoteFolder, GetDailyNoteTemplate, SetDailyNoteTemplate, GetFontFamily, GetFontSize, SetFontFamily, SetFontSize, GetPreviewFontFamily, GetPreviewFontSize, SetPreviewFontFamily, SetPreviewFontSize, ListThemes, LoadTheme, GetActiveTheme, SetActiveTheme, GetThemeDir, themeDir, ListTemplates, GetTemplateContent, SelectVault + `appConfig` 型 |
| `notes.go` | notePath, walkNotes, ListNotes, ListFolders, CreateFolder, RenameFolder, ReadNote, SaveNote, CreateNote, DeleteNote, RenameNote, updateWikilinks |
| `search.go` | SearchWithSnippets, SearchNotes, QueryNotes, GetTags, ListAllTags, GetTagCounts, SearchByTag + SearchHit/DataviewResult/TagCount 型 |
| `links.go` | GetBacklinks, GetBacklinksWithContext, GetGraph + BacklinkItem/GraphData 型 |
| `marp.go` | GetMarpTheme, SetMarpTheme, GetMarpThemesDir, ListMarpCustomThemes + MarpCustomTheme 型 |
| `export.go` | exportHTML, ExportHTML, ExportPDF, RenderMarkdown, inlineImages |
| `media.go` | resolveImagePath, uniqueAttachmentPath, SaveImage, SelectImage, OpenPath |
| `snippets.go` | GetSnippets, SaveSnippets + Snippet 型 |
| `terminal.go` | StartTerminal, WriteTerminal, ResizeTerminal, PrepareRunFile |
| `watcher.go` | startWatcher, onFileDrop |

型はそれを主に使うファイルへ。app.go に残るもの: App構造体、NewApp、startup。

- [ ] **Step 2: ビルド確認**

```bash
go build ./... && go vet ./...
```
Expected: エラーなし

- [ ] **Step 3: 実アプリ回帰確認**

`wails dev` を起動し、ブラウザ（またはこの後のsmoke script相当の手動確認）でノート一覧表示・ノート開閉・保存・検索が動くこと。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "refactor: app.goを機能単位の10ファイルに分割（純粋な移動）"
```

---

### Task 2: 回帰スモークスクリプト整備

**Files:**
- Create: `frontend/e2e/smoke.js`
- Modify: `frontend/package.json`（devDependencies に playwright-core）

**Interfaces:**
- Consumes: `wails dev` が localhost:34115 で稼働中であること
- Produces: `node frontend/e2e/smoke.js` — 以降の全タスクの回帰ゲート。exit 0 = 合格

- [ ] **Step 1: playwright-core を devDependency に追加**

```bash
cd frontend && npm i -D playwright-core
```

- [ ] **Step 2: smoke.js を作成**

```js
// 使い方: wails dev 起動後に node frontend/e2e/smoke.js
// chromium は ~/.cache/ms-playwright/chromium-*/chrome-linux64/chrome を使う
const { chromium } = require('playwright-core')
const fs = require('fs')
const os = require('os')

function findChromium() {
  const dir = os.homedir() + '/.cache/ms-playwright'
  const hit = fs.readdirSync(dir).filter((d) => /^chromium-\d+$/.test(d)).sort().pop()
  if (!hit) throw new Error('playwright chromium not found; npx playwright install chromium')
  return `${dir}/${hit}/chrome-linux64/chrome`
}

const fail = (msg) => { console.error('FAIL:', msg); process.exit(1) }

;(async () => {
  const browser = await chromium.launch({ executablePath: findChromium(), headless: true })
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } })
  const errors = []
  page.on('pageerror', (e) => errors.push(String(e)))

  await page.goto('http://localhost:34115', { waitUntil: 'networkidle' })
  await page.waitForTimeout(1500)

  // 1. 通常ノート: プレビューが描画される
  await page.locator('text=markdown-cheat-cheet').first().click()
  await page.waitForTimeout(1500)
  const htmlLen = await page.evaluate(() => document.querySelector('.preview')?.innerHTML.length ?? 0)
  if (htmlLen < 100) fail('normal preview empty')

  // 2. Marpノート: スライドとサムネイルが出る
  await page.locator('text=無題2').first().click()
  await page.waitForTimeout(4000)
  const marp = await page.evaluate(() => ({
    slides: document.querySelectorAll('.marp-slide-box').length,
    thumbs: document.querySelectorAll('.marp-thumb').length,
  }))
  if (marp.slides < 1 || marp.thumbs !== marp.slides) fail('marp render broken: ' + JSON.stringify(marp))

  // 3. ページエラーゼロ
  if (errors.length) fail('pageerrors: ' + JSON.stringify(errors))
  console.log('SMOKE PASS')
  await browser.close()
})().catch((e) => fail(e))
```
- [ ] **Step 3: 実行して合格を確認**

```bash
node frontend/e2e/smoke.js
```
Expected: `SMOKE PASS`

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/smoke.js frontend/package.json frontend/package-lock.json
git commit -m "test: Playwrightスモークスクリプトを追加"
```

---

### Task 3: lib/types.ts + lib/stores.ts + lib/markdown.ts の切り出し

**Files:**
- Create: `frontend/src/lib/types.ts`, `frontend/src/lib/stores.ts`, `frontend/src/lib/markdown.ts`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: なし
- Produces:
  - `types.ts`: App.svelte冒頭の共有型定義（`PaletteItem`, `SnippetDef`, `OutlineItem` 等、App.svelte内で `type`/`interface` 宣言されているもの全部）を無変更で移動しexport。Task 6/7がimportする
  - `stores.ts`: `export const toast: Writable<string>` / `export function showToast(msg: string): void` / `export const fontFamily, fontSize, previewFontFamily, previewFontSize: Writable<...>`
  - `markdown.ts`: 下記シグネチャ（後続タスクとApp.svelteが使う）

- [ ] **Step 1: stores.ts を作成**

```ts
import { writable } from 'svelte/store'

export const toast = writable('')
let toastTimer: ReturnType<typeof setTimeout>
export function showToast(msg: string): void {
  toast.set(msg)
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => toast.set(''), /* App.svelte showToast の既存タイムアウト値をそのまま */ 2000)
}

export const fontFamily = writable('')
export const fontSize = writable(16)
export const previewFontFamily = writable('')
export const previewFontSize = writable(16)
```
App.svelte の `showToast`/`toast`/`toastTimer` を削除し、`import { toast, showToast } from './lib/stores'` に置換。マークアップの `{#if toast}` は `{#if $toast}` に。フォント4変数も `$fontFamily` 等に置換し、`applyFont()` は `$:` リアクティブでstore値から適用する形に書き換え（適用ロジック自体は不変）。

- [ ] **Step 2: markdown.ts を作成し後処理関数を移す**

対象: `applyCheckboxes` / `applyMath` / `applyMermaid` / `applyDataview` / `dvEscape` / `applyCodeBlockButtons`（App.svelte 1084–1259 付近）。目標シグネチャ:

```ts
export function applyCheckboxes(root: HTMLElement, source: string, onSourceChange: (next: string) => void): void
export function applyMath(root: HTMLElement): void
export async function applyMermaid(root: HTMLElement, theme: 'dark' | 'light'): Promise<void>
export async function applyDataview(root: HTMLElement, onOpenNote: (name: string) => void): Promise<void>
export function applyCodeBlockButtons(root: HTMLElement, onRun: (lang: string, code: string) => void): void
```
`previewContentEl` 参照→`root`、`selectNote`→`onOpenNote`、`source`/保存→`onSourceChange` コールバック、mermaid初期化フラグ（`mermaidInitialized`）はモジュールローカル変数としてmarkdown.ts内へ。**移動前に各本体を読み、Appスコープ識別子が上記マッピングで尽きているか確認**。尽きていなければ引数を追加する。

- [ ] **Step 3: ビルド+回帰+コミット**

```bash
cd frontend && npx vite build   # 警告悪化なし・undefined/g ゼロ
node e2e/smoke.js               # SMOKE PASS（wails dev稼働下）
```
追加確認: チェックボックス付きノートでチェック切替→保存される、mermaid/KaTeX/Dataviewブロックが描画される。

```bash
git add -A && git commit -m "refactor: トースト/フォントをstoreへ、プレビュー後処理をlib/markdown.tsへ切り出し"
```

---

### Task 4: lib/editor.ts + lib/fuzzy.ts の切り出し

**Files:**
- Create: `frontend/src/lib/editor.ts`, `frontend/src/lib/fuzzy.ts`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: なし
- Produces:
  - `fuzzy.ts`: `export function fuzzyScore(query: string, str: string): number` / `export function fuzzyHighlight(query: string, str: string): string` / `export function highlightQuery(text: string, query: string): string`
  - `editor.ts`: 下記

- [ ] **Step 1: fuzzy.ts を作成**

`fuzzyScore` / `fuzzyHighlight` / `highlightQuery`（App.svelte 198–235付近）を無変更で移動。Appはimportに置換。

- [ ] **Step 2: editor.ts を作成**

対象と目標シグネチャ:

```ts
export function relativeLineNumbers(): Extension
export const livePreviewStyle: HighlightStyle
export function tableKeymap(): Extension        // isTableLine〜tableInsertRow は内部関数（export不要）
export function wikilinkPlugin(openFn: (name: string) => void): Extension
export function wikilinkCompletion(getNotes: () => string[]): /* 既存の戻り型を維持 */
export function setupVimGlobal(actions: Record<string, () => void>): void
export const vimModeField: StateField<string>
export const vimModeEffect: StateEffectType<string>
```
`setupVimGlobal` はVimコマンド→Appアクションの束なので、本体を読んで参照しているApp関数を列挙し、`actions` オブジェクトで受ける形に置換（キー名は関数名そのまま）。`wikilinkRe`/`wikilinkMark` はモジュールローカル。`initEditor` / `doSnippetExpand` / `handleImageFile` / 挿入系（wrapSelection等）はApp依存が濃いので**今回は移動しない**。

- [ ] **Step 3: ビルド+回帰+コミット**

smoke合格に加え: エディタで入力できる、`[[` でwikilink補完が出る、テーブル内Tabでセル移動、Vimモード切替が動く（設定 or ステータスバー）。

```bash
git add -A && git commit -m "refactor: CodeMirror拡張をlib/editor.tsへ、fuzzy検索をlib/fuzzy.tsへ切り出し"
```

---

### Task 5: MarpPreview.svelte の切り出し

**Files:**
- Create: `frontend/src/MarpPreview.svelte`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: なし（バックエンド SetMarpTheme / GetMarpThemesDir / OpenPath / ListMarpCustomThemes はコンポーネントが直接import）
- Produces:
  ```svelte
  <script context="module">
    export function detectMarp(src: string): boolean  // 既存実装をそのまま移動
  </script>
  props: export let source: string
  events: なし（プレゼン全画面も内部状態で完結）
  ```

- [ ] **Step 1: Marp一式を移動**

移動対象（シンボル）: getMarp, reloadCustomThemes, mountSection, scrollToMarpSlide, startThumbResize, observeSlide, changeMarpTheme, applyMarpTheme, detectMarp, renderMarpSlides と、marp* / _marp* / _lastMarpSrc / _lastRawSrc / customMarpThemes / _loadedCustomThemeNames の全状態変数、`trackPreviewWidth`+`previewWidth`（Marp専用ならコンポーネントへ。previewElにも使っていたら残す）。マークアップは `{#if isMarp && marpSections.length > 0}` ブロック全体（サムネストリップ・ツールバー含む）と `{#if isMarpFullscreen}` ブロック。CSSは `.marp-*` 全セレクタ。

App側は:
```svelte
$: isMarp = detectMarp(source)
...
{#if isMarp}
  <MarpPreview {source} />
{:else}
  （既存の通常プレビュー）
{/if}
```
`render()` 内のMarp分岐（renderMarpSlides呼び出しとデバウンス `_marpRenderTimer`）はコンポーネント内の `$: source` リアクティブへ移す。プレゼンの `window.addEventListener('message', ...)` （marp-exit/next/prev、onMount内 2076付近）もコンポーネントのonMountへ移動し、destroyで解除。

- [ ] **Step 2: ビルド+回帰**

smoke合格（marpスライド+サムネ検査を含む）に加えて手動確認: テーマボタン切替が効く、ストリップのドラッグリサイズ、プレゼン起動→矢印キー→Escで戻る、ノート（`MHB0621_最終発表版`）でノートパネル表示。

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "refactor: Marpプレビュー一式をMarpPreview.svelteへ切り出し"
```

---

### Task 6: Settings.svelte の切り出し

**Files:**
- Create: `frontend/src/Settings.svelte`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `stores.ts` のフォントstore（書き込む側）
- Produces:
  ```svelte
  props: export let themeList: string[]; export let activeTheme: string
  events: dispatch('close') / dispatch('themeSelect', name: string) /
          dispatch('vaultChange') / dispatch('snippetsChange', snippets: SnippetDef[])
  ```

- [ ] **Step 1: 設定モーダルを移動**

マークアップ `{#if showSettings}` ブロック（2493–2682付近）の中身をSettings.svelteへ。移動する状態/関数: settingsCategory, editingSnippet, newSnippet と、フォント・フォルダ・テンプレート設定のフォームロジック。バックエンド Get/Set（SetFontFamily, SetTemplatesFolder等）はコンポーネントが直接import。フォント変更はstoreに書く（Appの `$:` applyFontが反応）。テーマ選択・vault変更・スニペット保存はAppに残る処理（applyTheme / changeVault / snippets反映）があるためイベントで通知。App側は `{#if showSettings}<Settings ... on:close={() => showSettings = false} />{/if}`。CSSは設定モーダル系セレクタを同伴移動。

- [ ] **Step 2: ビルド+回帰**

smoke合格 + 手動: 設定を開く→フォントサイズ変更が即反映→再起動相当（リロード）で保持、テーマ切替、スニペット追加→エディタで展開。

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "refactor: 設定モーダルをSettings.svelteへ切り出し"
```

---

### Task 7: QuickSwitcher.svelte の切り出し

**Files:**
- Create: `frontend/src/QuickSwitcher.svelte`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `lib/fuzzy.ts`
- Produces:
  ```svelte
  props: export let notes: string[]; export let commands: PaletteItem[]
  events: dispatch('select', item: PaletteItem) / dispatch('close')
  ```
  `PaletteItem` 型は Task 3 で `lib/types.ts` に移動済み。ここからimportする。

- [ ] **Step 1: パレットを移動**

移動対象: qsQuery, qsIndex, qsInputEl, qsMode, qsSubValue, onQsKeydown, フィルタリングロジック、マークアップ `{#if showQuickSwitcher}` ブロック、CSS `.qs-*`。`executeItem`（コマンド実行の分岐）と `openQuickSwitcher`/`closeQuickSwitcher` の状態はAppに残す — コンポーネントは選択結果を `select` で返すだけ。rename/newFolderのサブ入力モード（qsMode）はUI状態なのでコンポーネント内、確定時に `select` のdetailにモードと入力値を含める（既存のexecuteItem分岐がそのまま受けられる形に）。

- [ ] **Step 2: ビルド+回帰**

smoke合格 + 手動: Ctrl+P（既存ショートカット、onGlobalKeydown参照）でパレット→ノート名fuzzy検索→Enterで開く、コマンド実行1種、Escで閉じる。

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "refactor: クイックスイッチャーをQuickSwitcher.svelteへ切り出し"
```

---

### Task 8: Sidebar.svelte の切り出し

**Files:**
- Create: `frontend/src/Sidebar.svelte`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `lib/fuzzy.ts`, `TreeItem.svelte`, バックエンド検索/CRUD API（直接import）
- Produces:
  ```svelte
  props: export let notes: string[]; export let folders: string[];
         export let currentNote: string | null; export let width: number
  events: dispatch('select', path: string) / dispatch('refresh') /
          dispatch('tagSelect', tag: string)
  ```

- [ ] **Step 1: サイドバーを移動**

移動対象: 検索（searchQuery, searchHits, runSearch, onSearchInput, searchOperators, hasOperator, fuzzyNameHits, visibleNotes計算）、ツリー（buildTree, expanded, onToggle）、rename（renamingPath他, startRename*, confirmRename, cancelRename, focusInput）、コンテキストメニュー（contextMenu, on*ContextMenu, closeContextMenu, createNoteAt, createFolderViaMenu, deleteNote）、移動（moveTargetNote, moveNoteTo, moveTo）、タグパネル（showTagPanel, tagCounts, selectTag→`tagSelect`イベント化）。マークアップ: `<main>` 直下のサイドバー領域（2150–2330付近）+ `{#if contextMenu}`（2740）+ `{#if moveTargetNote}`（2808）。CSS同伴。バックエンド呼び出し（DeleteNote, RenameNote等）は直接import、完了後 `dispatch('refresh')`。削除・rename後の「開いているノートだった場合の後始末」（clearCurrentNoteView相当）はAppの `refresh`/`select` ハンドラで行う。

- [ ] **Step 2: ビルド+回帰**

smoke合格 + 手動: ツリー開閉、検索（通常+`tag:`演算子）、右クリック→新規ノート/rename/削除、ノートのフォルダ移動、タグパネルからタグ選択。

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "refactor: サイドバーをSidebar.svelteへ切り出し"
```

---

### Task 9: 死にコード掃除と仕上げ

**Files:**
- Modify: `frontend/src/App.svelte`, 各新規ファイル
- Modify: `docs/superpowers/specs/2026-07-07-refactor-split-design.md`（ステータス更新）

- [ ] **Step 1: 未使用コードを検出して削除**

```bash
cd frontend && npx vite build 2>&1 | grep -i "unused"   # 未使用CSSセレクタ一覧（.rename-input 等）
npx svelte-check --threshold warning 2>&1 | head -50    # 未使用変数・型エラー
```
検出された未使用CSSセレクタ・未使用import・どこからも呼ばれない関数を削除。**確信が持てないものは消さない**（動的参照の可能性）。

- [ ] **Step 2: 行数確認**

```bash
wc -l frontend/src/App.svelte   # 目標: 1500行以下
wc -l app.go                    # 骨格のみ（~150行程度）
```
超過していても機械的に無理に削らない。実測を記録するだけ。

- [ ] **Step 3: 最終回帰+コミット**

smoke合格 + Task 5〜8の手動確認項目をひと通り再確認。specのステータスを「実装済み (YYYY-MM-DD)」に更新。

```bash
git add -A && git commit -m "refactor: 死にコード削除とリファクタ仕上げ"
```
