<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, basicSetup } from 'codemirror'
  import { EditorState } from '@codemirror/state'
  import { markdown } from '@codemirror/lang-markdown'
  import { oneDark } from '@codemirror/theme-one-dark'
  import TreeItem from './TreeItem.svelte'
  import type { TreeNode } from './TreeItem.svelte'
  import {
    RenderMarkdown,
    ListNotes,
    ListFolders,
    CreateFolder,
    RenameFolder,
    ReadNote,
    SaveNote,
    CreateNote,
    DeleteNote,
    RenameNote,
    SearchNotes,
    GetVaultPath,
    SelectVault,
    GetBacklinks,
  } from '../wailsjs/go/main/App.js'

  let notes: string[] = []
  let folders: string[] = []
  let visibleNotes: string[] = []
  let currentNote: string | null = null
  let source = ''
  let html = ''
  let backlinks: string[] = []
  let newNoteName = ''
  let saveTimer: ReturnType<typeof setTimeout>
  let renamingPath: string | null = null
  let renamingType: 'note' | 'folder' | null = null
  let renameValue = ''
  let expanded = new Set<string>()
  let searchQuery = ''
  let searchTimer: ReturnType<typeof setTimeout>
  let vaultPath = ''
  let newNoteInputEl: HTMLInputElement
  let searchInputEl: HTMLInputElement
  let saveStatus = ''
  let saveStatusTimer: ReturnType<typeof setTimeout>
  let contextMenu: { x: number; y: number; type: 'empty' | 'note' | 'folder'; path?: string } | null = null

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
      const node: TreeNode = { type: 'note', name: basename(n), path: n }
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
  $: isSearching = searchQuery.trim().length > 0

  function focusInput(el: HTMLInputElement): void {
    el.focus()
    el.select()
  }

  async function refreshList(): Promise<void> {
    notes = await ListNotes()
    folders = await ListFolders()
    await runSearch()
  }

  async function runSearch(): Promise<void> {
    visibleNotes = searchQuery.trim() ? await SearchNotes(searchQuery) : notes
  }

  function onSearchInput(): void {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(runSearch, 200)
  }

  async function selectNote(name: string): Promise<void> {
    source = await ReadNote(name)
    currentNote = name
    await render()
    backlinks = await GetBacklinks(name)
  }

  async function render(): Promise<void> {
    html = await RenderMarkdown(source)
  }

  function onEdit(): void {
    render()
    if (!currentNote) return
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => SaveNote(currentNote!, source), 400)
  }

  async function forceSave(): Promise<void> {
    if (!currentNote) return
    clearTimeout(saveTimer)
    await SaveNote(currentNote, source)
    saveStatus = '保存しました'
    clearTimeout(saveStatusTimer)
    saveStatusTimer = setTimeout(() => (saveStatus = ''), 1500)
  }

  function onGlobalKeydown(e: KeyboardEvent): void {
    if (!(e.ctrlKey || e.metaKey)) return
    if (e.key === 's') {
      e.preventDefault()
      forceSave()
    } else if (e.key === 'n') {
      e.preventDefault()
      newNoteInputEl?.focus()
    } else if (e.key === 'f') {
      e.preventDefault()
      searchInputEl?.focus()
    }
  }

  function initEditor(el: HTMLDivElement): { destroy(): void } {
    const view = new EditorView({
      state: EditorState.create({
        doc: source,
        extensions: [
          basicSetup,
          markdown(),
          oneDark,
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              source = update.state.doc.toString()
              onEdit()
            }
          }),
        ],
      }),
      parent: el,
    })
    return {
      destroy() {
        view.destroy()
      },
    }
  }

  async function newNote(): Promise<void> {
    const name = newNoteName.trim()
    if (!name) return
    await CreateNote(name)
    newNoteName = ''
    await refreshList()
    await selectNote(name)
  }

  function closeContextMenu(): void {
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

  async function createNoteAt(folderPath: string): Promise<void> {
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
    await refreshList()
    await selectNote(path)
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
    await refreshList()
    startRenameFolder(name)
  }

  async function deleteNote(name: string): Promise<void> {
    await DeleteNote(name)
    if (currentNote === name) {
      currentNote = null
      source = ''
      html = ''
      backlinks = []
    }
    await refreshList()
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
      if (currentNote === oldPath) currentNote = newPath
    }

    await refreshList()
    if (currentNote) {
      source = await ReadNote(currentNote)
      await render()
      backlinks = await GetBacklinks(currentNote)
    }
  }

  async function onPreviewClick(e: MouseEvent): Promise<void> {
    const target = (e.target as HTMLElement).closest('a')
    if (!target) return
    const href = target.getAttribute('href') ?? ''
    if (!href.startsWith('knote:')) return
    e.preventDefault()
    const name = href.slice('knote:'.length)
    if (!notes.includes(name)) {
      await CreateNote(name)
      await refreshList()
    }
    await selectNote(name)
  }

  async function changeVault(): Promise<void> {
    vaultPath = await SelectVault()
    currentNote = null
    source = ''
    html = ''
    backlinks = []
    searchQuery = ''
    await refreshList()
  }

  onMount(async () => {
    vaultPath = await GetVaultPath()
    await refreshList()
  })
</script>

<svelte:window on:keydown={onGlobalKeydown} on:click={closeContextMenu} />

<main>
  <nav class="sidebar">
    <div class="new-note">
      <input
        bind:this={newNoteInputEl}
        bind:value={newNoteName}
        on:keydown={(e) => e.key === 'Enter' && newNote()}
        placeholder="new note name"
      />
      <button on:click={newNote}>+</button>
    </div>
    <input
      bind:this={searchInputEl}
      class="search"
      bind:value={searchQuery}
      on:input={onSearchInput}
      placeholder="search"
    />
    <ul on:contextmenu={onSidebarContextMenu}>
      {#if isSearching}
        {#each visibleNotes as path}
          <li class:active={path === currentNote} on:contextmenu={(e) => onNoteContextMenu(e, path)}>
            {#if renamingPath === path && renamingType === 'note'}
              <input
                class="rename-input"
                use:focusInput
                bind:value={renameValue}
                on:keydown={(e) => {
                  if (e.key === 'Enter') confirmRename()
                  if (e.key === 'Escape') cancelRename()
                }}
                on:blur={confirmRename}
              />
            {:else}
              <span
                class="note-name"
                on:click={() => selectNote(path)}
                on:dblclick={() => startRename(path)}
              >{path}</span>
            {/if}
            <button class="delete" on:click={() => deleteNote(path)}>×</button>
          </li>
        {/each}
      {:else}
        {#each tree as node (node.path)}
          <TreeItem
            {node}
            depth={0}
            {currentNote}
            {renamingPath}
            {renamingType}
            bind:renameValue
            {expanded}
            onSelect={selectNote}
            {onToggle}
            onRenameStartNote={startRename}
            onRenameStartFolder={startRenameFolder}
            onConfirmRename={confirmRename}
            onCancelRename={cancelRename}
            onNoteContext={onNoteContextMenu}
            onFolderContext={onFolderContextMenu}
            onDeleteNote={deleteNote}
            focusInputAction={focusInput}
          />
        {/each}
      {/if}
    </ul>
    <div class="vault" title={vaultPath}>
      <span class="vault-path">{vaultPath}</span>
      <button on:click={changeVault}>変更</button>
    </div>
  </nav>

  {#if currentNote}
    {#key currentNote}
      <div class="editor" use:initEditor></div>
    {/key}
    <div class="preview">
      <div on:click={onPreviewClick}>{@html html}</div>
      {#if backlinks.length}
        <div class="backlinks">
          <div class="backlinks-title">バックリンク</div>
          <ul>
            {#each backlinks as name}
              <li><span class="note-name" on:click={() => selectNote(name)}>{name}</span></li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
  {:else}
    <div class="empty">ノートを選ぶか、新規作成して</div>
  {/if}

  {#if saveStatus}
    <div class="save-toast">{saveStatus}</div>
  {/if}

  {#if contextMenu}
    <div class="context-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px">
      {#if contextMenu.type === 'empty'}
        <button on:click={() => createNoteAt('')}>新規ノート</button>
        <button on:click={createFolderViaMenu}>新規フォルダ</button>
      {:else if contextMenu.type === 'note' && contextMenu.path}
        {@const path = contextMenu.path}
        <button on:click={() => createNoteAt(dirname(path))}>新規ノート</button>
        <button on:click={() => { closeContextMenu(); startRename(path) }}>名前を変更</button>
        <button on:click={() => { closeContextMenu(); deleteNote(path) }}>削除</button>
      {:else if contextMenu.type === 'folder' && contextMenu.path}
        {@const path = contextMenu.path}
        <button on:click={() => createNoteAt(path)}>新規ノート</button>
        <button on:click={() => { closeContextMenu(); startRenameFolder(path) }}>名前を変更</button>
      {/if}
    </div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
  }

  main {
    display: grid;
    grid-template-columns: 200px 1fr 1fr;
    height: 100vh;
  }

  .sidebar {
    border-right: 1px solid #ccc;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .new-note {
    display: flex;
    padding: 0.5rem;
    gap: 0.25rem;
  }

  .new-note input {
    flex: 1;
    min-width: 0;
  }

  .search {
    margin: 0 0.5rem 0.5rem;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    flex: 1;
    overflow-y: auto;
  }

  .vault {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.4rem 0.6rem;
    border-top: 1px solid #ccc;
    font-size: 0.75rem;
  }

  .vault-path {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.7;
  }

  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.6rem;
    cursor: pointer;
  }

  li.active {
    background: rgba(128, 128, 128, 0.2);
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

  .delete {
    border: none;
    background: none;
    cursor: pointer;
    opacity: 0.5;
  }

  .delete:hover {
    opacity: 1;
  }

  .editor {
    overflow: hidden;
    min-width: 0;
  }

  .editor :global(.cm-editor) {
    height: 100%;
  }

  .editor :global(.cm-scroller) {
    font-family: monospace;
    font-size: 1rem;
  }

  .preview {
    padding: 1rem;
    overflow-y: auto;
    border-left: 1px solid #ccc;
  }

  .preview :global(a[href^='knote:']) {
    color: #7c9eff;
    text-decoration: none;
    cursor: pointer;
  }

  .preview :global(a[href^='knote:']:hover) {
    text-decoration: underline;
  }

  .backlinks {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid #444;
  }

  .backlinks-title {
    font-size: 0.75rem;
    opacity: 0.6;
    margin-bottom: 0.4rem;
  }

  .backlinks ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .backlinks li {
    padding: 0.2rem 0;
  }

  .backlinks .note-name {
    color: #7c9eff;
  }

  .empty {
    grid-column: 2 / 4;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }

  .save-toast {
    position: fixed;
    top: 0.75rem;
    right: 0.75rem;
    background: rgba(124, 158, 255, 0.9);
    color: #1b2636;
    padding: 0.3rem 0.7rem;
    border-radius: 4px;
    font-size: 0.8rem;
  }

  .context-menu {
    position: fixed;
    display: flex;
    flex-direction: column;
    background: #2a3548;
    border: 1px solid #444;
    border-radius: 4px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    overflow: hidden;
    z-index: 10;
  }

  .context-menu button {
    border: none;
    background: none;
    color: inherit;
    text-align: left;
    padding: 0.4rem 1rem;
    cursor: pointer;
    white-space: nowrap;
  }

  .context-menu button:hover {
    background: rgba(124, 158, 255, 0.2);
  }
</style>
