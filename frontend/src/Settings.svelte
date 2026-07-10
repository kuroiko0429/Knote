<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import { X, FolderOpen, Sun, Moon } from 'lucide-svelte'
  import {
    GetTemplatesFolder,
    SetTemplatesFolder,
    SetDailyNoteFolder,
    SetDailyNoteTemplate,
    ListTemplates,
    ListThemes,
    SetFontFamily,
    SetFontSize,
    SetPreviewFontFamily,
    SetPreviewFontSize,
    GetSnippets,
    SaveSnippets,
  } from '../wailsjs/go/main/App.js'
  import type { SnippetDef } from './lib/types'
  import { fontFamily, fontSize, previewFontFamily, previewFontSize } from './lib/stores'

  export let themeList: string[] = []
  export let activeTheme = ''
  export let vaultPath = ''
  export let theme: 'dark' | 'light' = 'dark'
  export let vimMode = false
  export let dailyNoteFolder = ''
  export let dailyNoteTemplate = ''
  export let templateList: string[] = []

  const dispatch = createEventDispatcher<{
    close: void
    themeSelect: string
    vaultChange: void
    snippetsChange: SnippetDef[]
    toggleTheme: void
    toggleVim: void
    templatesChange: string[]
    dailyNoteChange: { folder: string; template: string }
  }>()

  let settingsCategory: 'general' | 'appearance' | 'templates' | 'snippets' = 'general'
  let editingSnippet: SnippetDef | null = null
  let newSnippet: SnippetDef = { trigger: '', name: '', content: '' }

  let templatesFolder = ''
  let snippets: SnippetDef[] = []

  let fontFamilyInput = ''
  let previewFontFamilyInput = ''
  $: fontFamilyInput = $fontFamily
  $: previewFontFamilyInput = $previewFontFamily

  onMount(async () => {
    templatesFolder = await GetTemplatesFolder()
    snippets = await GetSnippets()
  })

  function onThemeSelect(e: Event) {
    dispatch('themeSelect', (e.target as HTMLSelectElement).value)
  }

  function emitDailyNoteChange() {
    dispatch('dailyNoteChange', { folder: dailyNoteFolder, template: dailyNoteTemplate })
  }
</script>

<div class="modal-overlay" on:click={() => dispatch('close')}>
  <div class="settings-modal" on:click|stopPropagation>
    <button class="modal-close" on:click={() => dispatch('close')}><X size={18} /></button>
    <div class="settings-modal-body">
      <nav class="settings-nav">
        <button
          class:active={settingsCategory === 'general'}
          on:click={() => (settingsCategory = 'general')}
        >全般</button>
        <button
          class:active={settingsCategory === 'appearance'}
          on:click={async () => { settingsCategory = 'appearance'; themeList = await ListThemes() }}
        >外観</button>
        <button
          class:active={settingsCategory === 'templates'}
          on:click={() => (settingsCategory = 'templates')}
        >テンプレート</button>
        <button
          class:active={settingsCategory === 'snippets'}
          on:click={() => (settingsCategory = 'snippets')}
        >スニペット</button>
      </nav>
      <div class="settings-content">
        {#if settingsCategory === 'general'}
          <h3>全般</h3>
          <div class="settings-row">
            <FolderOpen size={14} />
            <span class="vault-path" title={vaultPath}>{vaultPath}</span>
            <button on:click={() => dispatch('vaultChange')}>変更</button>
          </div>
        {:else if settingsCategory === 'appearance'}
          <h3>外観</h3>
          <div class="settings-row">
            <button class="settings-action" on:click={() => dispatch('toggleTheme')}>
              {#if theme === 'dark'}<Sun size={14} />{:else}<Moon size={14} />{/if}
              テーマ：{theme === 'dark' ? 'ダーク' : 'ライト'}
            </button>
          </div>
          <div class="settings-row">
            <button class="settings-action" on:click={() => dispatch('toggleVim')}>
              Vimモード：{vimMode ? 'オン' : 'オフ'}
            </button>
          </div>
          <div class="settings-row settings-row-column">
            <span class="settings-label">カスタムテーマ</span>
            <select
              class="settings-select"
              value={activeTheme}
              on:change={onThemeSelect}
            >
              <option value="">なし（デフォルト）</option>
              {#each themeList as t}
                <option value={t}>{t}</option>
              {/each}
            </select>
          </div>
          <div class="settings-row settings-hint">
            テーマ CSS を <code>{vaultPath}/.knote/theme/</code> に配置
          </div>
          <div class="settings-font-section">
            <div class="settings-font-header">
              <span class="settings-label">エディタ</span>
              <button class="settings-sync-btn" on:click={async () => {
                $previewFontFamily = $fontFamily
                $previewFontSize = $fontSize
                await SetPreviewFontFamily($previewFontFamily)
                await SetPreviewFontSize($previewFontSize)
              }}>プレビューに同期</button>
            </div>
            <div class="settings-row settings-row-column">
              <span class="settings-label">フォント名</span>
              <input type="text" class="settings-input" placeholder="例: Noto Sans JP, monospace"
                bind:value={fontFamilyInput}
                on:change={async () => { $fontFamily = fontFamilyInput; await SetFontFamily($fontFamily) }} />
            </div>
            <div class="settings-row settings-row-column">
              <span class="settings-label">サイズ</span>
              <div class="settings-range-row">
                <input type="range" min="10" max="24" step="1" bind:value={$fontSize}
                  on:change={async () => { await SetFontSize($fontSize) }} />
                <span>{$fontSize || 14}px</span>
              </div>
            </div>
          </div>
          <div class="settings-font-section">
            <div class="settings-font-header">
              <span class="settings-label">プレビュー</span>
            </div>
            <div class="settings-row settings-row-column">
              <span class="settings-label">フォント名</span>
              <input type="text" class="settings-input" placeholder="例: Noto Sans JP, sans-serif"
                bind:value={previewFontFamilyInput}
                on:change={async () => { $previewFontFamily = previewFontFamilyInput; await SetPreviewFontFamily($previewFontFamily) }} />
            </div>
            <div class="settings-row settings-row-column">
              <span class="settings-label">サイズ</span>
              <div class="settings-range-row">
                <input type="range" min="10" max="24" step="1" bind:value={$previewFontSize}
                  on:change={async () => { await SetPreviewFontSize($previewFontSize) }} />
                <span>{$previewFontSize || 15}px</span>
              </div>
            </div>
          </div>
        {:else if settingsCategory === 'templates'}
          <h3>テンプレート</h3>
          <div class="settings-row">
            <span>テンプレートフォルダ</span>
            <input
              type="text"
              bind:value={templatesFolder}
              on:change={async () => {
                await SetTemplatesFolder(templatesFolder)
                templateList = await ListTemplates()
                dispatch('templatesChange', templateList)
              }}
            />
          </div>
          <div class="settings-row">
            <span>デイリーノートフォルダ</span>
            <input
              type="text"
              bind:value={dailyNoteFolder}
              on:change={() => { SetDailyNoteFolder(dailyNoteFolder); emitDailyNoteChange() }}
            />
          </div>
          <div class="settings-row">
            <span>デイリーノートのテンプレート</span>
            <select
              bind:value={dailyNoteTemplate}
              on:change={() => { SetDailyNoteTemplate(dailyNoteTemplate); emitDailyNoteChange() }}
            >
              <option value="">なし</option>
              {#each templateList as name}
                <option value={name}>{name}</option>
              {/each}
            </select>
          </div>
          <p class="settings-hint">使える変数: <code>{'{{date}}'}</code> <code>{'{{time}}'}</code> <code>{'{{title}}'}</code> <code>{'{{cursor}}'}</code></p>
        {:else if settingsCategory === 'snippets'}
          <h3>スニペット</h3>
          <p class="settings-hint">エディタでトリガーを入力して Tab を押すと展開。変数: <code>{'{{date}}'}</code> <code>{'{{time}}'}</code> <code>{'{{title}}'}</code> <code>{'{{cursor}}'}</code></p>
          <table class="snippet-table">
            <thead><tr><th>トリガー</th><th>名前</th><th>内容</th><th></th></tr></thead>
            <tbody>
              {#each snippets as snip, i}
                <tr>
                  {#if editingSnippet === snip}
                    <td><input type="text" bind:value={snip.trigger} /></td>
                    <td><input type="text" bind:value={snip.name} /></td>
                    <td><textarea rows="6" bind:value={snip.content}></textarea></td>
                    <td>
                      <button on:click={async () => { editingSnippet = null; await SaveSnippets(snippets); snippets = await GetSnippets(); dispatch('snippetsChange', snippets) }}>保存</button>
                      <button on:click={() => { editingSnippet = null }}>キャンセル</button>
                    </td>
                  {:else}
                    <td><code>{snip.trigger}</code></td>
                    <td>{snip.name}</td>
                    <td class="snippet-preview">{snip.content.slice(0, 40)}{snip.content.length > 40 ? '…' : ''}</td>
                    <td>
                      <button on:click={() => { editingSnippet = snip }}>編集</button>
                      <button on:click={async () => { snippets = snippets.filter((_, j) => j !== i); await SaveSnippets(snippets); dispatch('snippetsChange', snippets) }}>削除</button>
                    </td>
                  {/if}
                </tr>
              {/each}
              <tr class="snippet-new-row">
                <td><input type="text" placeholder="トリガー" bind:value={newSnippet.trigger} /></td>
                <td><input type="text" placeholder="名前" bind:value={newSnippet.name} /></td>
                <td><textarea rows="6" placeholder={"内容（{{cursor}}でカーソル位置指定）"} bind:value={newSnippet.content}></textarea></td>
                <td>
                  <button on:click={async () => {
                    if (!newSnippet.trigger || !newSnippet.content) return
                    snippets = [...snippets, { ...newSnippet }]
                    newSnippet = { trigger: '', name: '', content: '' }
                    await SaveSnippets(snippets)
                    dispatch('snippetsChange', snippets)
                  }}>追加</button>
                </td>
              </tr>
            </tbody>
          </table>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .settings-modal {
    position: relative;
    width: 800px;
    max-width: 92vw;
    height: 580px;
    max-height: 88vh;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    overflow: hidden;
  }

  .modal-close {
    position: absolute;
    top: 0.6rem;
    right: 0.6rem;
    display: flex;
    align-items: center;
    border: none;
    background: none;
    color: var(--text-dim);
    cursor: pointer;
    z-index: 1;
  }

  .modal-close:hover {
    color: var(--text);
  }

  .settings-modal-body {
    display: flex;
    height: 100%;
  }

  .settings-nav {
    width: 160px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    padding: 1rem 0.5rem;
    border-right: 1px solid var(--border);
    background: var(--bg-secondary);
  }

  .settings-nav button {
    text-align: left;
    border: none;
    background: none;
    color: var(--text-dim);
    border-radius: 4px;
    padding: 0.5rem 0.6rem;
    cursor: pointer;
  }

  .settings-nav button:hover {
    background: var(--bg-hover);
  }

  .settings-nav button.active {
    background: var(--accent-hover);
    color: var(--text);
  }

  .settings-content {
    flex: 1;
    padding: 1.5rem;
    overflow-y: auto;
  }

  .settings-content h3 {
    margin-top: 0;
  }

  .settings-row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem;
  }

  .vault-path {
    max-width: 40vw;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.7;
  }

  .settings-row .vault-path {
    flex: 1;
    max-width: none;
  }

  .settings-row > span {
    flex: 1;
  }

  .settings-row input[type='text'],
  .settings-row select {
    -webkit-appearance: none;
    appearance: none;
    padding: 0.3rem 0.5rem;
    background: var(--bg-secondary) !important;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text) !important;
    font-size: 0.85rem;
  }

  .settings-row input[type='text']:focus,
  .settings-row select:focus {
    outline: none;
    border-color: var(--accent);
  }

  .settings-row > button:not(.settings-action) {
    padding: 0.25rem 0.7rem;
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 4px;
    font-size: 0.8rem;
    cursor: pointer;
    white-space: nowrap;
  }

  .settings-row > button:not(.settings-action):hover {
    background: var(--accent-hover);
    color: var(--text);
  }

  .settings-action {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    border: none;
    background: none;
    color: var(--text);
    cursor: pointer;
    border-radius: 4px;
    padding: 0.4rem 0.2rem;
  }

  .settings-action:hover {
    background: var(--bg-hover);
  }

  .settings-label {
    font-size: 0.82rem;
    color: var(--text-dim);
    margin-bottom: 0.25rem;
    display: block;
  }

  .settings-select {
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
    background: var(--bg-secondary) !important;
    color: var(--text) !important;
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.35rem 0.5rem;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .settings-select:focus {
    outline: none;
    border-color: var(--accent);
  }

  .settings-font-section {
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.5rem;
  }
  .settings-font-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.4rem;
    font-weight: 600;
    font-size: 0.82rem;
  }
  .settings-sync-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 4px;
    padding: 0.15rem 0.5rem;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .settings-sync-btn:hover {
    opacity: 0.85;
  }
  .settings-range-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
  }
  .settings-range-row input[type="range"] {
    flex: 1;
  }
  .settings-range-row span {
    min-width: 2.5rem;
    text-align: right;
    font-size: 0.82rem;
    color: var(--text-dim);
  }

  .settings-input {
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
    background: var(--bg-secondary) !important;
    color: var(--text) !important;
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.35rem 0.5rem;
    font-size: 0.85rem;
  }
  .settings-input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .settings-row-column {
    flex-direction: column;
    align-items: stretch;
  }

  .settings-hint {
    font-size: 0.75rem;
    color: var(--text-dim);
    flex-direction: column;
    align-items: flex-start;
    gap: 0.2rem;
  }

  .settings-hint code {
    font-size: 0.72rem;
    background: var(--code-bg);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    word-break: break-all;
  }

  .snippet-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    margin-top: 0.5rem;
  }

  .snippet-table th {
    text-align: left;
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid var(--border);
    color: var(--text-dim);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .snippet-table td {
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid var(--border);
    vertical-align: top;
  }

  .snippet-table input[type="text"] {
    width: 100%;
    box-sizing: border-box;
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 4px;
    padding: 0.2rem 0.4rem;
    font-size: 0.8rem;
  }

  .snippet-table textarea {
    width: 100%;
    box-sizing: border-box;
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 4px;
    padding: 0.2rem 0.4rem;
    font-size: 0.8rem;
    font-family: monospace;
    resize: vertical;
  }

  .snippet-preview {
    color: var(--text-dim);
    font-family: monospace;
    font-size: 0.75rem;
    white-space: pre;
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .snippet-new-row td {
    background: var(--bg-hover);
  }
</style>
