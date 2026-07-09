<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import TreeItem from './TreeItem.svelte'
  import type { TreeNode } from './TreeItem.svelte'
  import { Search, FileText, Tag, FilePlus, FolderPlus, Pencil, Trash2, FolderInput } from 'lucide-svelte'
  import {
    CreateNote,
    CreateFolder,
    DeleteNote,
    RenameNote,
    RenameFolder,
    SearchNotes,
    SearchWithSnippets,
    SearchByTag,
    GetTagCounts,
  } from '../wailsjs/go/main/App.js'
  import { fuzzyScore, fuzzyHighlight, highlightQuery } from './lib/fuzzy'

  export let notes: string[] = []
  export let folders: string[] = []
  export let currentNote: string | null = null
  export let width: number = 200
  export let compactMode = false

  type RefreshDetail =
    | { kind: 'pathChange'; type: 'note' | 'folder'; oldPath: string; newPath: string }
    | { kind: 'moveModal'; oldPath: string; newPath: string }
    | { kind: 'delete'; path: string }
    | undefined

  const dispatch = createEventDispatcher<{
    select: string
    refresh: RefreshDetail
    tagSelect: string
  }>()

  let renamingPath: string | null = null
  let renamingType: 'note' | 'folder' | null = null
  let renameValue = ''
  let expanded = new Set<string>()
  let searchQuery = ''
  let searchTimer: ReturnType<typeof setTimeout>
  let searchSeq = 0
  let searchBusy = false
  let moveTargetNote: string | null = null
  let searchInputEl: HTMLInputElement
  const searchOperators = ['tag:', 'file:', 'path:', 'line:', 'section:']
  let searchHits: { path: string; snippets: string[] }[] = []
  const hasOperator = (q: string) => searchOperators.some((op) => q.includes(op))

  let fuzzyNameHits = new Set<string>()
  let visibleNotes: string[] = []

  let contextMenu: { x: number; y: number; type: 'empty' | 'note' | 'folder'; path?: string } | null = null

  let activeTag: string | null = null
  let tagCounts: { tag: string; count: number }[] = []
  let showTagPanel = false

  function basename(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? path : path.slice(i + 1)
  }

  function dirname(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? '' : path.slice(0, i)
  }

  function buildTree(folderPaths: string[], notePaths: string[]): TreeNode[] {
    const folderMap = new Map<string, TreeNode & { type: 'folder' }>()
    const rootChildren: TreeNode[] = []

    function getOrCreateFolder(path: string): TreeNode & { type: 'folder' } {
      let node = folderMap.get(path)
      if (node) return node
      node = { type: 'folder', name: basename(path), path, children: [] }
      folderMap.set(path, node)
      const parent = dirname(path)
      if (parent) getOrCreateFolder(parent).children.push(node)
      else rootChildren.push(node)
      return node
    }

    for (const f of [...folderPaths].sort((a, b) => a.split('/').length - b.split('/').length)) {
      getOrCreateFolder(f)
    }

    for (const n of notePaths) {
      const conflict = folderMap.has(n)
      const node: TreeNode = { type: 'note', name: basename(n), path: n, ...(conflict && { conflict: true }) }
      const parent = dirname(n)
      if (parent) getOrCreateFolder(parent).children.push(node)
      else rootChildren.push(node)
    }

    function sortChildren(nodes: TreeNode[]): void {
      nodes.sort((a, b) => {
        if (a.type !== b.type) return a.type === 'folder' ? -1 : 1
        return a.name.localeCompare(b.name)
      })
      for (const n of nodes) if (n.type === 'folder') sortChildren(n.children)
    }
    sortChildren(rootChildren)

    return rootChildren
  }

  $: tree = buildTree(folders, notes)
  $: isFiltering = searchQuery.trim().length > 0 || activeTag !== null

  function focusInput(el: HTMLInputElement): void {
    el.focus()
    el.select()
  }

  async function loadTagCounts(): Promise<void> {
    tagCounts = await GetTagCounts()
  }

  $: { void notes; void folders; loadTagCounts() }
  $: if (notes) runSearch()

  async function runSearch(): Promise<void> {
    if (searchBusy) return
    searchBusy = true
    const seq = ++searchSeq
    const q = searchQuery.trim()
    try {
      let hits: { path: string; snippets: string[] }[] = []
      let paths: string[] = []
      let nextFuzzyHits = new Set<string>()
      if (q) {
        activeTag = null
        if (q.startsWith('#')) {
          const tag = q.slice(1)
          paths = tag ? await SearchByTag(tag) : notes
        } else if (hasOperator(q)) {
          paths = await SearchNotes(q)
        } else {
          const base = (p: string) => { const i = p.lastIndexOf('/'); return i === -1 ? p : p.slice(i + 1) }
          const fuzzyRanked = notes
            .map(p => ({ path: p, score: Math.max(fuzzyScore(q, base(p)), fuzzyScore(q, p)) }))
            .filter(m => m.score >= 0)
            .sort((a, b) => b.score - a.score)
          hits = await SearchWithSnippets(q)
          const contentSet = new Set(hits.map(h => h.path))
          const fuzzyOnly = fuzzyRanked.filter(m => !contentSet.has(m.path)).map(m => m.path)
          nextFuzzyHits = new Set(fuzzyRanked.map(m => m.path))
          paths = [...fuzzyOnly, ...hits.map(h => h.path)]
        }
      } else if (activeTag) {
        paths = await SearchByTag(activeTag)
      } else {
        paths = notes
      }
      if (seq === searchSeq) {
        searchHits = hits
        visibleNotes = paths
        fuzzyNameHits = nextFuzzyHits
      }
    } finally {
      searchBusy = false
      if (seq !== searchSeq) runSearch()
    }
  }

  function onSearchInput(): void {
    ++searchSeq
    clearTimeout(searchTimer)
    searchTimer = setTimeout(runSearch, 350)
  }

  export async function selectTag(tag: string): Promise<void> {
    if (activeTag === tag) {
      activeTag = null
    } else {
      activeTag = tag
      searchQuery = ''
    }
    await runSearch()
    dispatch('tagSelect', tag)
  }

  export function resetFilters(): void {
    searchQuery = ''
    activeTag = null
    runSearch()
  }

  export function focusSearch(): void {
    searchInputEl?.focus()
  }

  export function expandFolder(path: string): void {
    if (!expanded.has(path)) expanded = new Set(expanded).add(path)
  }

  function selectPath(path: string): void {
    dispatch('select', path)
  }

  export function closeContextMenu(): void {
    contextMenu = null
  }

  function onSidebarContextMenu(e: MouseEvent): void {
    e.preventDefault()
    contextMenu = { x: e.clientX, y: e.clientY, type: 'empty' }
  }

  function onNoteContextMenu(e: MouseEvent, path: string): void {
    e.preventDefault()
    e.stopPropagation()
    contextMenu = { x: e.clientX, y: e.clientY, type: 'note', path }
  }

  function onFolderContextMenu(e: MouseEvent, path: string): void {
    e.preventDefault()
    e.stopPropagation()
    contextMenu = { x: e.clientX, y: e.clientY, type: 'folder', path }
  }

  function onToggle(path: string): void {
    const next = new Set(expanded)
    if (next.has(path)) next.delete(path)
    else next.add(path)
    expanded = next
  }

  export async function createNoteAt(folderPath: string): Promise<void> {
    closeContextMenu()
    let base = '無題'
    let i = 1
    let path = folderPath ? `${folderPath}/${base}` : base
    while (notes.includes(path)) {
      base = `無題${i}`
      path = folderPath ? `${folderPath}/${base}` : base
      i++
    }
    await CreateNote(path)
    if (folderPath && !expanded.has(folderPath)) {
      expanded = new Set(expanded).add(folderPath)
    }
    dispatch('refresh')
    dispatch('select', path)
    startRename(path)
  }

  async function createFolderViaMenu(): Promise<void> {
    closeContextMenu()
    let name = '新規フォルダ'
    let i = 1
    while (folders.includes(name)) {
      name = `新規フォルダ${i}`
      i++
    }
    await CreateFolder(name)
    dispatch('refresh')
    startRenameFolder(name)
  }

  export async function deleteNote(name: string): Promise<void> {
    await DeleteNote(name)
    dispatch('refresh', { kind: 'delete', path: name })
  }

  function startRename(path: string): void {
    renamingPath = path
    renamingType = 'note'
    renameValue = basename(path)
  }

  function startRenameFolder(path: string): void {
    renamingPath = path
    renamingType = 'folder'
    renameValue = basename(path)
  }

  function cancelRename(): void {
    renamingPath = null
    renamingType = null
  }

  async function confirmRename(): Promise<void> {
    const oldPath = renamingPath
    const type = renamingType
    const newBase = renameValue.trim()
    renamingPath = null
    renamingType = null
    if (!oldPath || !newBase) return
    const dir = dirname(oldPath)
    const newPath = dir ? `${dir}/${newBase}` : newBase
    if (newPath === oldPath) return

    if (type === 'folder') {
      await RenameFolder(oldPath, newPath)
      if (expanded.has(oldPath)) {
        const next = new Set(expanded)
        next.delete(oldPath)
        next.add(newPath)
        expanded = next
      }
    } else {
      await RenameNote(oldPath, newPath)
    }

    dispatch('refresh', { kind: 'pathChange', type: type === 'folder' ? 'folder' : 'note', oldPath, newPath })
  }

  async function moveNoteTo(notePath: string, destFolder: string): Promise<void> {
    moveTargetNote = null
    const name = basename(notePath)
    const newPath = destFolder ? `${destFolder}/${name}` : name
    if (newPath === notePath) return
    await RenameNote(notePath, newPath)
    dispatch('refresh', { kind: 'moveModal', oldPath: notePath, newPath })
  }

  async function moveTo(targetFolder: string, e: DragEvent): Promise<void> {
    e.preventDefault()
    e.stopPropagation()
    const data = e.dataTransfer?.getData('text/plain')
    if (!data) return
    const { path, type } = JSON.parse(data) as { path: string; type: 'note' | 'folder' }

    const newPath = targetFolder ? `${targetFolder}/${basename(path)}` : basename(path)
    if (newPath === path) return
    if (type === 'folder' && (targetFolder === path || targetFolder.startsWith(path + '/'))) return

    if (type === 'folder') {
      await RenameFolder(path, newPath)
      if (expanded.has(path)) {
        const next = new Set(expanded)
        next.delete(path)
        next.add(newPath)
        expanded = next
      }
    } else {
      await RenameNote(path, newPath)
    }

    if (targetFolder && !expanded.has(targetFolder)) {
      expanded = new Set(expanded).add(targetFolder)
    }

    dispatch('refresh', { kind: 'pathChange', type, oldPath: path, newPath })
  }
</script>

<nav class="sidebar" class:compact-mode={compactMode}>
  <div class="search-box">
    <Search size={14} class="search-icon" />
    <input
      bind:this={searchInputEl}
      class="search"
      bind:value={searchQuery}
      on:input={onSearchInput}
      placeholder="search"
    />
  </div>
  <ul
    on:contextmenu={onSidebarContextMenu}
    on:dragover={(e) => e.preventDefault()}
    on:drop={(e) => moveTo('', e)}
  >
    {#if isFiltering}
      {#each visibleNotes as path}
        {@const hit = searchHits.find((h) => h.path === path)}
        {@const q = searchQuery.trim()}
        {@const base = path.includes('/') ? path.slice(path.lastIndexOf('/') + 1) : path}
        {@const dir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) + ' / ' : ''}
        <li
          class="search-result"
          class:active={path === currentNote}
          on:click={() => selectPath(path)}
          on:contextmenu={(e) => onNoteContextMenu(e, path)}
        >
          <span class="note-name">
            <FileText size={13} />
            {#if dir}<span class="search-dir">{dir}</span>{/if}
            {#if fuzzyNameHits.has(path) && !q.startsWith('#')}
              {@html fuzzyHighlight(q, base)}
            {:else}
              {base}
            {/if}
          </span>
          {#if hit}
            <ul class="snippets">
              {#each hit.snippets as snippet}
                <li>{@html highlightQuery(snippet, q)}</li>
              {/each}
            </ul>
          {/if}
        </li>
      {/each}
    {:else}
      {#each tree as node (node.type + ':' + node.path)}
        <TreeItem
          {node}
          depth={0}
          {currentNote}
          {renamingPath}
          {renamingType}
          bind:renameValue
          {expanded}
          onSelect={selectPath}
          {onToggle}
          onRenameStartNote={startRename}
          onRenameStartFolder={startRenameFolder}
          onConfirmRename={confirmRename}
          onCancelRename={cancelRename}
          onNoteContext={onNoteContextMenu}
          onFolderContext={onFolderContextMenu}
          onDrop={moveTo}
          focusInputAction={focusInput}
        />
      {/each}
    {/if}
  </ul>

  {#if tagCounts.length > 0}
    <div class="tag-panel">
      <button class="tag-panel-header" on:click={() => showTagPanel = !showTagPanel}>
        <Tag size={13} />
        <span>タグ</span>
        <span class="tag-panel-chevron">{showTagPanel ? '▲' : '▼'}</span>
      </button>
      {#if showTagPanel}
        <div class="tag-cloud">
          {#each tagCounts as { tag, count }}
            {@const maxCount = tagCounts[0].count}
            {@const size = 11 + Math.round((count / maxCount) * 6)}
            <button
              class="tag-cloud-chip"
              class:active={activeTag === tag}
              style="font-size:{size}px"
              on:click={() => selectTag(tag)}
              title="{tag} ({count})"
            >{tag} <span class="tag-count">{count}</span></button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</nav>

{#if contextMenu}
  <div class="context-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px">
    {#if contextMenu.type === 'empty'}
      <button on:click={() => createNoteAt('')}><FilePlus size={14} /> 新規ノート</button>
      <button on:click={createFolderViaMenu}><FolderPlus size={14} /> 新規フォルダ</button>
    {:else if contextMenu.type === 'note' && contextMenu.path}
      {@const path = contextMenu.path}
      <button on:click={() => createNoteAt(dirname(path))}><FilePlus size={14} /> 新規ノート</button>
      <button on:click={() => { closeContextMenu(); startRename(path) }}><Pencil size={14} /> 名前を変更</button>
      <button on:click={() => { closeContextMenu(); moveTargetNote = path }}><FolderInput size={14} /> 移動...</button>
      <button on:click={() => { closeContextMenu(); deleteNote(path) }}><Trash2 size={14} /> 削除</button>
    {:else if contextMenu.type === 'folder' && contextMenu.path}
      {@const path = contextMenu.path}
      <button on:click={() => createNoteAt(path)}><FilePlus size={14} /> 新規ノート</button>
      <button on:click={() => { closeContextMenu(); startRenameFolder(path) }}><Pencil size={14} /> 名前を変更</button>
    {/if}
  </div>
{/if}

{#if moveTargetNote}
  {@const _mtn = moveTargetNote}
  <div class="modal-overlay" on:click={() => (moveTargetNote = null)}>
    <div class="move-modal" on:click|stopPropagation>
      <div class="move-modal-title"><FolderInput size={15} /> 移動先を選択</div>
      <div class="move-modal-note">{_mtn}</div>
      <ul class="move-folder-list">
        <li>
          <button class="move-folder-btn" on:click={() => moveNoteTo(_mtn, '')}>
            <span class="move-folder-icon">🏠</span> ルート
          </button>
        </li>
        {#each folders as folder}
          {@const isCurrent = dirname(_mtn) === folder}
          <li>
            <button class="move-folder-btn" class:move-current={isCurrent}
              on:click={() => moveNoteTo(_mtn, folder)}>
              <span class="move-folder-icon">📁</span> {folder}
            </button>
          </li>
        {/each}
      </ul>
      <button class="move-cancel" on:click={() => (moveTargetNote = null)}>キャンセル</button>
    </div>
  </div>
{/if}

<style>
  .sidebar {
    grid-column: 1;
    grid-row: 2 / 5;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .search-box {
    position: relative;
    margin: 0.5rem 0.5rem;
  }

  .search-box :global(.search-icon) {
    position: absolute;
    left: 0.5rem;
    top: 50%;
    transform: translateY(-50%);
    opacity: 0.5;
    pointer-events: none;
  }


  .search {
    width: 100%;
    box-sizing: border-box;
    padding: 0.4rem 0.6rem 0.4rem 1.8rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
  }

  .search:focus {
    outline: none;
    border-color: var(--accent);
  }

  .sidebar.compact-mode {
    font-size: 0.8rem;
  }

  .sidebar.compact-mode .search-box {
    padding: 0.3rem 0.5rem;
  }

  .sidebar.compact-mode .search {
    font-size: 0.78rem;
  }

  .tag-panel {
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .tag-panel-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    background: none;
    border: none;
    color: var(--text-dim);
    font-size: 0.75rem;
    padding: 0.4rem 0.75rem;
    cursor: pointer;
    text-align: left;
  }

  .tag-panel-header:hover {
    color: var(--text);
    background: var(--bg-hover);
  }

  .tag-panel-chevron {
    margin-left: auto;
    font-size: 0.6rem;
  }

  .tag-cloud {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    padding: 0.4rem 0.75rem 0.6rem;
    max-height: 160px;
    overflow-y: auto;
  }

  .tag-cloud-chip {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-dim);
    border-radius: 10px;
    padding: 0.1rem 0.45rem;
    cursor: pointer;
    line-height: 1.5;
    transition: background 0.1s;
  }

  .tag-cloud-chip:hover {
    background: var(--bg-hover);
    color: var(--text);
  }

  .tag-cloud-chip.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }

  .tag-count {
    font-size: 0.65em;
    opacity: 0.7;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    flex: 1;
    overflow-y: auto;
  }

  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.6rem;
    cursor: pointer;
  }

  li.active {
    background: var(--bg-hover);
  }

  .note-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .rename-input {
    flex: 1;
    min-width: 0;
  }

  .search-result {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    justify-content: flex-start;
    gap: 0.2rem;
    padding: 0.4rem 0.6rem;
    cursor: pointer;
    border-radius: 4px;
  }

  .search-result:hover,
  .search-result.active {
    background: var(--bg-hover);
  }

  .search-result .note-name {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.82rem;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .search-dir {
    font-weight: 400;
    opacity: 0.5;
    font-size: 0.78rem;
  }

  .search-result .note-name :global(mark) {
    background: var(--accent);
    color: var(--accent-contrast);
    border-radius: 2px;
    padding: 0 1px;
  }

  .snippets {
    list-style: none;
    margin: 0;
    padding: 0 0 0 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .snippets li {
    font-size: 0.75rem;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 1.4;
  }

  .snippets :global(mark) {
    background: rgba(255, 200, 0, 0.35);
    color: inherit;
    border-radius: 2px;
    padding: 0 1px;
  }

  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 20;
  }

  .move-modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 1.2rem;
    width: 320px;
    max-height: 420px;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }

  .move-modal-title {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: 600;
    font-size: 0.9rem;
  }

  .move-modal-note {
    font-size: 0.78rem;
    color: var(--text-dim);
    padding: 0.3rem 0.5rem;
    background: var(--bg);
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .move-folder-list {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .move-folder-btn {
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: var(--text);
    padding: 0.4rem 0.6rem;
    border-radius: 5px;
    cursor: pointer;
    font-size: 0.85rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .move-folder-btn:hover { background: var(--bg-hover); }
  .move-folder-btn.move-current { opacity: 0.4; cursor: default; }
  .move-folder-icon { font-size: 0.9rem; }

  .move-cancel {
    align-self: flex-end;
    background: none;
    border: 1px solid var(--border);
    color: var(--text-dim);
    padding: 0.3rem 0.8rem;
    border-radius: 5px;
    cursor: pointer;
    font-size: 0.82rem;
  }

  .move-cancel:hover { background: var(--bg-hover); }

  .context-menu {
    position: fixed;
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    overflow: hidden;
    z-index: 10;
  }

  .context-menu button {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    border: none;
    background: none;
    color: inherit;
    text-align: left;
    padding: 0.4rem 1rem;
    cursor: pointer;
    white-space: nowrap;
  }

  .context-menu button:hover {
    background: var(--accent-hover);
  }
</style>
