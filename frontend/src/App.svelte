<script lang="ts">
  import { onMount } from 'svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import { EditorView, lineNumbers, highlightSpecialChars, drawSelection, dropCursor, highlightActiveLine, keymap } from '@codemirror/view'
  import { EditorState, EditorSelection } from '@codemirror/state'
  import { history, defaultKeymap, historyKeymap } from '@codemirror/commands'
  import { markdown } from '@codemirror/lang-markdown'
  import { languages } from '@codemirror/language-data'
  import { oneDark } from '@codemirror/theme-one-dark'
  import { HighlightStyle, syntaxHighlighting, defaultHighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language'
  import { tags } from '@lezer/highlight'
  import TreeItem from './TreeItem.svelte'
  import type { TreeNode } from './TreeItem.svelte'
  import GraphView from './GraphView.svelte'
  import TerminalPanel from './Terminal.svelte'
  import {
    FilePlus,
    FolderPlus,
    Search,
    Network,
    FolderOpen,
    Check,
    Pencil,
    Trash2,
    Link2,
    X,
    NotebookText,
    Sun,
    Moon,
    TerminalSquare,
    Tag,
    Bold,
    Italic,
    Strikethrough,
    Heading2,
    Link as LinkIcon,
    Code,
    List,
    Quote,
    Columns2,
    PanelLeft,
    PanelRight,
  } from 'lucide-svelte'
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
    GetGraph,
    GetTags,
    SearchByTag,
  } from '../wailsjs/go/main/App.js'

  let notes: string[] = []
  let folders: string[] = []
  let visibleNotes: string[] = []
  let showGraph = false
  let graphEdges: { source: string; target: string }[] = []
  let currentNote: string | null = null
  let source = ''
  let html = ''
  let backlinks: string[] = []
  let saveTimer: ReturnType<typeof setTimeout>
  let renamingPath: string | null = null
  let renamingType: 'note' | 'folder' | null = null
  let renameValue = ''
  let expanded = new Set<string>()
  let searchQuery = ''
  let searchTimer: ReturnType<typeof setTimeout>
  let vaultPath = ''
  let searchInputEl: HTMLInputElement
  let searchFocused = false
  let searchBlurTimer: ReturnType<typeof setTimeout>
  const searchOperators = ['tag:', 'file:', 'path:', 'line:', 'section:']

  function onSearchFocus(): void {
    clearTimeout(searchBlurTimer)
    searchFocused = true
  }

  function onSearchBlur(): void {
    searchBlurTimer = setTimeout(() => (searchFocused = false), 150)
  }

  function insertSearchOperator(op: string): void {
    searchQuery = searchQuery.trim() ? `${searchQuery.trim()} ${op}` : op
    searchInputEl?.focus()
    onSearchInput()
  }
  let saveStatus = ''
  let saveStatusTimer: ReturnType<typeof setTimeout>
  let contextMenu: { x: number; y: number; type: 'empty' | 'note' | 'folder'; path?: string } | null = null
  let theme: 'dark' | 'light' = (localStorage.getItem('knote-theme') as 'dark' | 'light' | null) ?? 'dark'
  let activeTag: string | null = null
  let noteTags: string[] = []
  let viewMode: 'split' | 'editor' | 'preview' = 'split'

  function toggleTheme(): void {
    theme = theme === 'dark' ? 'light' : 'dark'
    localStorage.setItem('knote-theme', theme)
  }

  $: if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', theme)
  }

  let showTerminal = false

  function toggleTerminal(): void {
    showTerminal = !showTerminal
  }

  let mainEl: HTMLElement
  let terminalPanelEl: HTMLDivElement
  let sidebarWidth = 200
  let editorWidth = 400
  let terminalHeight = 260
  let dragging: 'sidebar' | 'editor' | 'terminal' | null = null

  onMount(() => {
    if (mainEl) {
      editorWidth = Math.max(200, (mainEl.clientWidth - sidebarWidth) / 2)
    }
  })

  function startDrag(which: 'sidebar' | 'editor' | 'terminal', e: PointerEvent): void {
    dragging = which
    document.body.style.userSelect = 'none'
    e.preventDefault()
  }

  function onDragMove(e: PointerEvent): void {
    if (!dragging || !mainEl) return
    if (dragging === 'sidebar') {
      sidebarWidth = Math.max(140, Math.min(480, e.clientX))
    } else if (dragging === 'editor') {
      const maxEditor = mainEl.clientWidth - sidebarWidth - 200
      editorWidth = Math.max(200, Math.min(maxEditor, e.clientX - sidebarWidth))
    } else if (dragging === 'terminal' && terminalPanelEl) {
      const rect = terminalPanelEl.getBoundingClientRect()
      terminalHeight = Math.max(120, Math.min(600, rect.bottom - e.clientY))
    }
  }

  function endDrag(): void {
    dragging = null
    document.body.style.userSelect = ''
  }

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
  $: isFiltering = searchQuery.trim().length > 0 || activeTag !== null
  $: breadcrumb = currentNote ? currentNote.split('/').join(' / ') : ''

  function countChars(htmlStr: string): number {
    const div = document.createElement('div')
    div.innerHTML = htmlStr
    return (div.textContent || '').replace(/\s/g, '').length
  }

  $: charCount = currentNote ? countChars(html) : 0

  function focusInput(el: HTMLInputElement): void {
    el.focus()
    el.select()
  }

  async function refreshList(): Promise<void> {
    notes = await ListNotes()
    folders = await ListFolders()
    await runSearch()
    if (showGraph) graphEdges = (await GetGraph()).edges
  }

  async function toggleGraph(): Promise<void> {
    showGraph = !showGraph
    if (showGraph) graphEdges = (await GetGraph()).edges
  }

  async function onGraphSelect(e: CustomEvent<string>): Promise<void> {
    const name = e.detail
    showGraph = false
    if (!notes.includes(name)) {
      await CreateNote(name)
      await refreshList()
    }
    await selectNote(name)
  }

  async function runSearch(): Promise<void> {
    if (searchQuery.trim()) {
      activeTag = null
      visibleNotes = await SearchNotes(searchQuery)
    } else if (activeTag) {
      visibleNotes = await SearchByTag(activeTag)
    } else {
      visibleNotes = notes
    }
  }

  function onSearchInput(): void {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(runSearch, 200)
  }

  async function selectTag(tag: string): Promise<void> {
    if (activeTag === tag) {
      activeTag = null
    } else {
      activeTag = tag
      searchQuery = ''
    }
    await runSearch()
  }

  async function selectNote(name: string): Promise<void> {
    source = await ReadNote(name)
    currentNote = name
    await render()
    backlinks = await GetBacklinks(name)
    noteTags = await GetTags(name)
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
      createNoteAt('')
    } else if (e.key === 'f') {
      e.preventDefault()
      searchInputEl?.focus()
    }
  }

  const livePreviewStyle = HighlightStyle.define([
    { tag: tags.heading1, fontSize: '1.6em', fontWeight: 'bold' },
    { tag: tags.heading2, fontSize: '1.4em', fontWeight: 'bold' },
    { tag: tags.heading3, fontSize: '1.25em', fontWeight: 'bold' },
    { tag: tags.heading4, fontSize: '1.1em', fontWeight: 'bold' },
    { tag: tags.heading5, fontWeight: 'bold' },
    { tag: tags.heading6, fontWeight: 'bold', opacity: '0.85' },
    { tag: tags.strong, fontWeight: 'bold' },
    { tag: tags.emphasis, fontStyle: 'italic' },
    { tag: tags.strikethrough, textDecoration: 'line-through' },
    { tag: tags.monospace, fontFamily: 'monospace', backgroundColor: 'var(--code-bg)' },
    { tag: tags.link, color: 'var(--accent)' },
    { tag: tags.url, color: 'var(--accent)', textDecoration: 'underline' },
    { tag: tags.quote, fontStyle: 'italic', opacity: '0.8' },
    { tag: tags.processingInstruction, opacity: '0.4' },
  ])

  let editorView: EditorView | null = null

  function initEditor(el: HTMLDivElement): { destroy(): void } {
    const view = new EditorView({
      state: EditorState.create({
        doc: source,
        extensions: [
          lineNumbers(),
          highlightSpecialChars(),
          history(),
          drawSelection(),
          dropCursor(),
          EditorState.allowMultipleSelections.of(true),
          indentOnInput(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          bracketMatching(),
          highlightActiveLine(),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          markdown({ codeLanguages: languages }),
          ...(theme === 'dark' ? [oneDark] : []),
          syntaxHighlighting(livePreviewStyle),
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
    editorView = view
    return {
      destroy() {
        view.destroy()
        if (editorView === view) editorView = null
      },
    }
  }

  function wrapSelection(before: string, after: string = before): void {
    if (!editorView) return
    const view = editorView
    const tr = view.state.changeByRange((range) => {
      const changes = [
        { from: range.from, insert: before },
        { from: range.to, insert: after },
      ]
      const newFrom = range.from + before.length
      const newTo = range.to + before.length
      return {
        changes,
        range: range.empty ? EditorSelection.cursor(newFrom) : EditorSelection.range(newFrom, newTo),
      }
    })
    view.dispatch(view.state.update(tr))
    view.focus()
  }

  function prefixLines(prefix: string): void {
    if (!editorView) return
    const view = editorView
    const tr = view.state.changeByRange((range) => {
      const startLine = view.state.doc.lineAt(range.from)
      return {
        changes: { from: startLine.from, insert: prefix },
        range: EditorSelection.range(range.from + prefix.length, range.to + prefix.length),
      }
    })
    view.dispatch(view.state.update(tr))
    view.focus()
  }

  function insertLink(): void {
    if (!editorView) return
    const view = editorView
    const tr = view.state.changeByRange((range) => {
      const text = view.state.doc.sliceString(range.from, range.to)
      const label = text || 'リンク'
      const insert = `[${label}](url)`
      return {
        changes: [{ from: range.from, to: range.to, insert }],
        range: EditorSelection.range(range.from + label.length + 3, range.from + label.length + 6),
      }
    })
    view.dispatch(view.state.update(tr))
    view.focus()
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
      noteTags = []
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
      noteTags = await GetTags(currentNote)
    }
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
      if (currentNote === path) currentNote = newPath
    }

    if (targetFolder && !expanded.has(targetFolder)) {
      expanded = new Set(expanded).add(targetFolder)
    }

    await refreshList()
    if (currentNote) {
      source = await ReadNote(currentNote)
      await render()
      backlinks = await GetBacklinks(currentNote)
      noteTags = await GetTags(currentNote)
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
    noteTags = []
    searchQuery = ''
    activeTag = null
    await refreshList()
  }

  async function onVaultChanged(): Promise<void> {
    await refreshList()
    if (currentNote && !notes.includes(currentNote)) {
      currentNote = null
      source = ''
      html = ''
      backlinks = []
      noteTags = []
    } else if (currentNote) {
      backlinks = await GetBacklinks(currentNote)
      noteTags = await GetTags(currentNote)
    }
  }

  onMount(async () => {
    vaultPath = await GetVaultPath()
    await refreshList()
    EventsOn('vault:changed', onVaultChanged)
  })
</script>

<svelte:window
  on:keydown={onGlobalKeydown}
  on:click={closeContextMenu}
  on:pointermove={onDragMove}
  on:pointerup={endDrag}
/>

<main bind:this={mainEl} style="grid-template-columns: {sidebarWidth}px {editorWidth}px 1fr">
  <header class="topbar">
    <span class="app-title"><NotebookText size={16} /> Knote</span>
    <div class="topbar-right">
      {#if breadcrumb}<span class="breadcrumb">{breadcrumb}</span>{/if}
      {#if currentNote && !showGraph}
        <div class="view-mode-toggle">
          <button
            class:active={viewMode === 'editor'}
            on:click={() => (viewMode = 'editor')}
            title="エディタのみ"
          ><PanelLeft size={15} /></button>
          <button
            class:active={viewMode === 'split'}
            on:click={() => (viewMode = 'split')}
            title="分割表示"
          ><Columns2 size={15} /></button>
          <button
            class:active={viewMode === 'preview'}
            on:click={() => (viewMode = 'preview')}
            title="プレビューのみ"
          ><PanelRight size={15} /></button>
        </div>
      {/if}
      <button on:click={toggleTheme} title="テーマ切り替え">
        {#if theme === 'dark'}<Sun size={16} />{:else}<Moon size={16} />{/if}
      </button>
      <button on:click={toggleGraph} title="グラフ">
        {#if showGraph}<X size={16} />{:else}<Network size={16} />{/if}
      </button>
    </div>
  </header>

  <nav class="sidebar">
    <div class="search-box">
      <Search size={14} class="search-icon" />
      <input
        bind:this={searchInputEl}
        class="search"
        bind:value={searchQuery}
        on:input={onSearchInput}
        on:focus={onSearchFocus}
        on:blur={onSearchBlur}
        placeholder="search"
      />
      {#if searchFocused}
        <div class="search-hints">
          {#each searchOperators as op}
            <button on:click={() => insertSearchOperator(op)}>{op}</button>
          {/each}
        </div>
      {/if}
    </div>
    <ul
      on:contextmenu={onSidebarContextMenu}
      on:dragover={(e) => e.preventDefault()}
      on:drop={(e) => moveTo('', e)}
    >
      {#if isFiltering}
        {#each visibleNotes as path}
          <li
            class:active={path === currentNote}
            draggable="true"
            on:dragstart={(e) => {
              e.dataTransfer?.setData('text/plain', JSON.stringify({ path, type: 'note' }))
              if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
            }}
            on:dragover={(e) => e.preventDefault()}
            on:drop={(e) => moveTo(dirname(path), e)}
            on:contextmenu={(e) => onNoteContextMenu(e, path)}
          >
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
            onDrop={moveTo}
            focusInputAction={focusInput}
          />
        {/each}
      {/if}
    </ul>
  </nav>

  <div
    class="resize-handle resize-handle-v"
    style="left: {sidebarWidth - 2}px"
    on:pointerdown={(e) => startDrag('sidebar', e)}
  ></div>
  {#if currentNote && !showGraph && viewMode === 'split'}
    <div
      class="resize-handle resize-handle-v"
      style="left: {sidebarWidth + editorWidth - 2}px"
      on:pointerdown={(e) => startDrag('editor', e)}
    ></div>
  {/if}

  {#if showGraph}
    <div class="graph-view">
      <GraphView {notes} edges={graphEdges} {currentNote} on:select={onGraphSelect} />
    </div>
  {:else if currentNote}
    <div class="editor" class:full={viewMode === 'editor'} class:hidden={viewMode === 'preview'}>
      <div class="editor-toolbar">
        <button on:click={() => wrapSelection('**')} title="太字"><Bold size={15} /></button>
        <button on:click={() => wrapSelection('*')} title="斜体"><Italic size={15} /></button>
        <button on:click={() => wrapSelection('~~')} title="取り消し線"><Strikethrough size={15} /></button>
        <button on:click={() => wrapSelection('`')} title="インラインコード"><Code size={15} /></button>
        <button on:click={() => prefixLines('## ')} title="見出し"><Heading2 size={15} /></button>
        <button on:click={insertLink} title="リンク"><LinkIcon size={15} /></button>
        <button on:click={() => prefixLines('- ')} title="箇条書き"><List size={15} /></button>
        <button on:click={() => prefixLines('> ')} title="引用"><Quote size={15} /></button>
      </div>
      {#key currentNote + theme}
        <div class="editor-mount" use:initEditor></div>
      {/key}
    </div>
    <div class="preview" class:full={viewMode === 'preview'} class:hidden={viewMode === 'editor'}>
      {#if noteTags.length}
        <div class="note-tags">
          {#each noteTags as tag}
            <button class="tag-chip" on:click={() => selectTag(tag)}><Tag size={11} />{tag}</button>
          {/each}
        </div>
      {/if}
      <div on:click={onPreviewClick}>{@html html}</div>
      {#if backlinks.length}
        <div class="backlinks">
          <div class="backlinks-title"><Link2 size={13} /> バックリンク</div>
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

  <div
    class="terminal-panel"
    class:hidden={!showTerminal}
    bind:this={terminalPanelEl}
    style="height: {terminalHeight}px"
  >
    <div
      class="resize-handle resize-handle-h"
      on:pointerdown={(e) => startDrag('terminal', e)}
    ></div>
    <div class="terminal-panel-header">
      <span><TerminalSquare size={13} /> ターミナル</span>
      <button on:click={toggleTerminal}><X size={14} /></button>
    </div>
    <TerminalPanel visible={showTerminal} />
  </div>

  <footer class="bottombar">
    <div class="bottombar-left" title={vaultPath}>
      <FolderOpen size={14} />
      <span class="vault-path">{vaultPath}</span>
      <button on:click={changeVault} title="保存先を変更">変更</button>
    </div>
    <div class="bottombar-right">
      {#if saveStatus}
        <span class="save-status"><Check size={13} /> {saveStatus}</span>
      {/if}
      {#if currentNote}
        <span class="char-count">{charCount}文字</span>
      {/if}
      <button on:click={toggleTerminal} title="ターミナル" class:active={showTerminal}>
        <TerminalSquare size={14} />
      </button>
    </div>
  </footer>

  {#if contextMenu}
    <div class="context-menu" style="left:{contextMenu.x}px; top:{contextMenu.y}px">
      {#if contextMenu.type === 'empty'}
        <button on:click={() => createNoteAt('')}><FilePlus size={14} /> 新規ノート</button>
        <button on:click={createFolderViaMenu}><FolderPlus size={14} /> 新規フォルダ</button>
      {:else if contextMenu.type === 'note' && contextMenu.path}
        {@const path = contextMenu.path}
        <button on:click={() => createNoteAt(dirname(path))}><FilePlus size={14} /> 新規ノート</button>
        <button on:click={() => { closeContextMenu(); startRename(path) }}><Pencil size={14} /> 名前を変更</button>
        <button on:click={() => { closeContextMenu(); deleteNote(path) }}><Trash2 size={14} /> 削除</button>
      {:else if contextMenu.type === 'folder' && contextMenu.path}
        {@const path = contextMenu.path}
        <button on:click={() => createNoteAt(path)}><FilePlus size={14} /> 新規ノート</button>
        <button on:click={() => { closeContextMenu(); startRenameFolder(path) }}><Pencil size={14} /> 名前を変更</button>
      {/if}
    </div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
  }

  main {
    position: relative;
    display: grid;
    grid-template-rows: auto 1fr auto auto;
    height: 100vh;
  }

  .resize-handle {
    position: absolute;
    z-index: 5;
  }

  .resize-handle-v {
    top: 0;
    bottom: 0;
    width: 5px;
    cursor: col-resize;
  }

  .resize-handle-v:hover {
    background: var(--accent-hover);
  }

  .resize-handle-h {
    top: -3px;
    left: 0;
    right: 0;
    height: 5px;
    cursor: row-resize;
    z-index: 6;
  }

  .resize-handle-h:hover {
    background: var(--accent-hover);
  }

  .topbar {
    grid-column: 1 / 4;
    grid-row: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.8rem;
    border-bottom: 1px solid var(--border);
  }

  .app-title {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: bold;
  }

  .topbar-right {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .topbar-right button {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    border: 1px solid var(--border);
    background: var(--bg-secondary);
    color: var(--text);
    border-radius: 6px;
    padding: 0.3rem 0.5rem;
    cursor: pointer;
    transition: background-color 0.15s, border-color 0.15s;
  }

  .topbar-right button:hover {
    background: var(--accent-hover);
    border-color: var(--accent);
  }

  .view-mode-toggle {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }

  .view-mode-toggle button {
    border: none;
    border-radius: 0;
    background: var(--bg-secondary);
    color: var(--text-dim);
  }

  .view-mode-toggle button + button {
    border-left: 1px solid var(--border);
  }

  .view-mode-toggle button.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .breadcrumb {
    font-size: 0.8rem;
    opacity: 0.6;
  }

  .sidebar {
    grid-column: 1;
    grid-row: 2 / 4;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    min-height: 0;
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

  .search-hints {
    position: absolute;
    top: calc(100% + 0.3rem);
    left: 0;
    right: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    padding: 0.4rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    z-index: 5;
  }

  .search-hints button {
    border: 1px solid var(--border);
    background: var(--bg);
    color: var(--text-dim);
    border-radius: 10px;
    padding: 0.1rem 0.5rem;
    font-size: 0.7rem;
    font-family: monospace;
    cursor: pointer;
  }

  .search-hints button:hover {
    color: var(--text);
    border-color: var(--accent);
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

  .tag-chip {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    border: 1px solid var(--border);
    background: var(--bg-secondary);
    color: var(--text-dim);
    border-radius: 10px;
    padding: 0.1rem 0.5rem;
    font-size: 0.7rem;
    cursor: pointer;
  }

  .tag-chip:hover {
    color: var(--text);
  }

  .note-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-bottom: 1rem;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    flex: 1;
    overflow-y: auto;
  }

  .vault-path {
    max-width: 40vw;
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


  .editor {
    grid-row: 2;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }

  .editor.full {
    grid-column: 2 / 4;
  }

  .editor.hidden {
    display: none;
  }

  .editor-toolbar {
    display: flex;
    align-items: center;
    gap: 0.2rem;
    padding: 0.3rem 0.5rem;
    border-bottom: 1px solid var(--border);
  }

  .editor-toolbar button {
    display: flex;
    align-items: center;
    border: none;
    background: none;
    color: var(--text-dim);
    border-radius: 4px;
    padding: 0.3rem;
    cursor: pointer;
  }

  .editor-toolbar button:hover {
    background: var(--bg-hover);
    color: var(--text);
  }

  .editor-mount {
    flex: 1;
    min-height: 0;
  }

  .editor-mount :global(.cm-editor) {
    height: 100%;
  }

  .editor-mount :global(.cm-scroller) {
    font-family: monospace;
    font-size: 1rem;
  }

  .preview {
    grid-row: 2;
    padding: 1rem;
    overflow-y: auto;
    border-left: 1px solid var(--border);
  }

  .preview.full {
    grid-column: 2 / 4;
  }

  .preview.hidden {
    display: none;
  }

  .preview :global(a[href^='knote:']) {
    color: var(--accent);
    text-decoration: none;
    cursor: pointer;
  }

  .preview :global(a[href^='knote:']:hover) {
    text-decoration: underline;
  }

  .preview :global(table) {
    border-collapse: collapse;
    margin: 1rem 0;
  }

  .preview :global(th),
  .preview :global(td) {
    border: 1px solid var(--border);
    padding: 0.4rem 0.7rem;
  }

  .preview :global(th) {
    background: var(--code-bg);
  }

  .backlinks {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
  }

  .backlinks-title {
    display: flex;
    align-items: center;
    gap: 0.3rem;
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
    color: var(--accent);
  }

  .empty {
    grid-column: 2 / 4;
    grid-row: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }

  .graph-view {
    grid-column: 2 / 4;
    grid-row: 2;
    min-height: 0;
    overflow: hidden;
  }

  .bottombar {
    grid-column: 1 / 4;
    grid-row: 4;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.3rem 0.8rem;
    border-top: 1px solid var(--border);
    font-size: 0.75rem;
  }

  .terminal-panel {
    position: relative;
    grid-column: 2 / 4;
    grid-row: 3;
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--border);
    border-left: 1px solid var(--border);
    background: #1b2636;
  }

  .terminal-panel.hidden {
    display: none;
  }

  .terminal-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.25rem 0.6rem;
    font-size: 0.75rem;
    color: var(--text-dim);
    border-bottom: 1px solid var(--border);
  }

  .terminal-panel-header span {
    display: flex;
    align-items: center;
    gap: 0.3rem;
  }

  .terminal-panel-header button {
    border: none;
    background: none;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
  }

  .bottombar-left {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-width: 0;
  }

  .bottombar-left button {
    border: none;
    background: none;
    color: inherit;
    cursor: pointer;
    opacity: 0.7;
  }

  .bottombar-left button:hover {
    opacity: 1;
  }

  .bottombar-right {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    opacity: 0.7;
  }

  .bottombar-right button {
    display: flex;
    align-items: center;
    border: none;
    background: none;
    color: inherit;
    cursor: pointer;
    opacity: 0.85;
  }

  .bottombar-right button.active {
    color: var(--accent);
    opacity: 1;
  }

  .save-status {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    color: var(--accent);
  }

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
