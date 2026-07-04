<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import { EditorView, lineNumbers, highlightSpecialChars, drawSelection, dropCursor, highlightActiveLine, keymap } from '@codemirror/view'
  import { EditorState, EditorSelection } from '@codemirror/state'
  import { history, defaultKeymap, historyKeymap, undo, redo } from '@codemirror/commands'
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
    Settings,
    FileText,
    AlignLeft,
    Image,
    Underline,
    Code2,
    ListOrdered,
    ListChecks,
    Minus,
    Table,
    Undo2,
    Redo2,
    Calendar,
    FileStack,
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
    SaveImage,
    SelectImage,
    GetTemplatesFolder,
    SetTemplatesFolder,
    GetDailyNoteFolder,
    SetDailyNoteFolder,
    GetDailyNoteTemplate,
    SetDailyNoteTemplate,
    ListTemplates,
    GetTemplateContent,
  } from '../wailsjs/go/main/App.js'

  let notes: string[] = []
  let folders: string[] = []
  let visibleNotes: string[] = []
  let showGraph = false
  let graphEdges: { source: string; target: string }[] = []
  let currentNote: string | null = null
  let openTabs: string[] = []
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
  let templatesFolder = ''
  let dailyNoteFolder = ''
  let dailyNoteTemplate = ''
  let templateList: string[] = []
  let showTemplatePicker = false
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
  let showSettings = false
  let settingsCategory: 'general' | 'appearance' | 'templates' = 'general'
  let showQuickSwitcher = false
  let qsQuery = ''
  let qsIndex = 0
  let qsInputEl: HTMLInputElement

  $: qsResults = qsQuery.trim()
    ? notes.filter((n) => n.toLowerCase().includes(qsQuery.trim().toLowerCase()))
    : notes
  $: qsExactMatch = notes.includes(qsQuery.trim())
  $: qsShowCreate = qsQuery.trim().length > 0 && !qsExactMatch
  $: qsTotal = qsResults.length + (qsShowCreate ? 1 : 0)
  $: if (qsIndex >= qsTotal) qsIndex = Math.max(0, qsTotal - 1)

  function openQuickSwitcher(): void {
    showQuickSwitcher = true
    qsQuery = ''
    qsIndex = 0
    tick().then(() => qsInputEl?.focus())
  }

  function closeQuickSwitcher(): void {
    showQuickSwitcher = false
  }

  async function qsSelect(path: string): Promise<void> {
    closeQuickSwitcher()
    if (!notes.includes(path)) {
      await CreateNote(path)
      await refreshList()
    }
    await openTab(path)
  }

  function onQsKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault()
      closeQuickSwitcher()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      qsIndex = Math.min(qsIndex + 1, qsTotal - 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      qsIndex = Math.max(qsIndex - 1, 0)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (qsIndex < qsResults.length) {
        qsSelect(qsResults[qsIndex])
      } else if (qsShowCreate) {
        qsSelect(qsQuery.trim())
      }
    }
  }

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
  $: breadcrumb = currentNote ? currentNote.split('/').join(' / ') : ''

  function countChars(src: string): number {
    const stripped = src.replace(/^---\n[\s\S]*?\n---\n?/, '')
    return stripped.replace(/\s/g, '').length
  }

  $: charCount = currentNote ? countChars(source) : 0
  $: lineCount = currentNote ? source.split('\n').length : 0
  $: readingMins = charCount > 0 ? Math.max(1, Math.round(charCount / 500)) : 0

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
    await openTab(name)
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

  async function openTab(path: string): Promise<void> {
    if (currentNote) {
      clearTimeout(saveTimer)
      await SaveNote(currentNote, source)
    }
    if (!openTabs.includes(path)) {
      openTabs = [...openTabs, path]
    }
    await selectNote(path)
  }

  function closeTab(path: string, e?: MouseEvent): void {
    e?.stopPropagation()
    const idx = openTabs.indexOf(path)
    if (idx === -1) return
    openTabs = openTabs.filter((p) => p !== path)
    if (currentNote !== path) return

    const next = openTabs[idx] ?? openTabs[idx - 1]
    if (next) {
      openTab(next)
    } else {
      currentNote = null
      source = ''
      html = ''
      backlinks = []
      noteTags = []
      outline = []
    }
  }

  async function render(): Promise<void> {
    html = await RenderMarkdown(source)
    await updateOutline()
  }

  interface OutlineItem {
    level: number
    text: string
    el: HTMLElement
  }

  let outline: OutlineItem[] = []
  let showOutline = false
  let previewContentEl: HTMLDivElement

  async function updateOutline(): Promise<void> {
    await tick()
    if (!previewContentEl) {
      outline = []
      return
    }
    const headings = Array.from(previewContentEl.querySelectorAll('h1, h2, h3, h4, h5, h6')) as HTMLElement[]
    outline = headings.map((el) => ({ level: Number(el.tagName[1]), text: el.textContent || '', el }))
  }

  function jumpToHeading(item: OutlineItem): void {
    item.el.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
    if (e.key === 'Escape' && showSettings) {
      showSettings = false
      return
    }
    if (e.key === 'Escape' && showQuickSwitcher) {
      closeQuickSwitcher()
      return
    }
    if (!(e.ctrlKey || e.metaKey)) return
    if (e.key === 's') {
      e.preventDefault()
      forceSave()
    } else if (e.key === 'n') {
      e.preventDefault()
      createNoteAt('')
    } else if (e.key === 'p') {
      e.preventDefault()
      openQuickSwitcher()
    } else if (e.key === 'f') {
      e.preventDefault()
      searchInputEl?.focus()
    } else if (e.key === 'd') {
      e.preventDefault()
      openDailyNote()
    }
  }

  function todayDateString(): string {
    const now = new Date()
    const yyyy = now.getFullYear()
    const mm = String(now.getMonth() + 1).padStart(2, '0')
    const dd = String(now.getDate()).padStart(2, '0')
    return `${yyyy}-${mm}-${dd}`
  }

  async function openDailyNote(): Promise<void> {
    const date = todayDateString()
    const folder = dailyNoteFolder || 'daily'
    const path = `${folder}/${date}`
    if (!notes.includes(path)) {
      await CreateNote(path)
      let content = `# ${date}\n\n`
      if (dailyNoteTemplate) {
        try {
          content = (await GetTemplateContent(dailyNoteTemplate)).split('{{date}}').join(date)
        } catch {
          // template missing; fall back to the default header
        }
      }
      await SaveNote(path, content)
      expanded = new Set(expanded).add(folder)
      await refreshList()
    }
    await openTab(path)
  }

  async function loadTemplateList(): Promise<void> {
    templateList = await ListTemplates()
  }

  async function insertTemplate(name: string): Promise<void> {
    showTemplatePicker = false
    if (!editorView) return
    const content = (await GetTemplateContent(name)).split('{{date}}').join(todayDateString())
    insertAtCursor(content)
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

  async function handleImageFile(file: File, view: EditorView, pos: number): Promise<void> {
    const reader = new FileReader()
    const base64 = await new Promise<string>((resolve, reject) => {
      reader.onload = () => resolve((reader.result as string).split(',')[1] ?? '')
      reader.onerror = reject
      reader.readAsDataURL(file)
    })
    const relPath = await SaveImage(file.name, base64)
    const insert = `![](${relPath})`
    view.dispatch({
      changes: { from: pos, insert },
      selection: { anchor: pos + insert.length },
    })
  }

  async function insertImageFromFile(): Promise<void> {
    if (!editorView) return
    const view = editorView
    const pos = view.state.selection.main.from
    const relPath = await SelectImage()
    if (!relPath) return
    const insert = `![](${relPath})`
    view.dispatch({
      changes: { from: pos, insert },
      selection: { anchor: pos + insert.length },
    })
  }

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
          EditorView.domEventHandlers({
            paste(event, view) {
              const item = Array.from(event.clipboardData?.items ?? []).find((i) => i.type.startsWith('image/'))
              const file = item?.getAsFile()
                ?? Array.from(event.clipboardData?.files ?? []).find((f) => f.type.startsWith('image/'))
              if (!file) return false
              event.preventDefault()
              handleImageFile(file, view, view.state.selection.main.from)
              return true
            },
            drop(event, view) {
              // Always swallow drops: external file drops are handled natively
              // via Wails' OnFileDrop (see "image:dropped" listener below),
              // since WebKitGTK doesn't reliably expose dataTransfer.files here
              // and would otherwise fall back to inserting the raw file path.
              event.preventDefault()
              const file = Array.from(event.dataTransfer?.files ?? []).find((f) => f.type.startsWith('image/'))
              if (file) {
                const pos = view.posAtCoords({ x: event.clientX, y: event.clientY }) ?? view.state.selection.main.from
                handleImageFile(file, view, pos)
              }
              return true
            },
          }),
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

  function insertAtCursor(text: string): void {
    if (!editorView) return
    const view = editorView
    const pos = view.state.selection.main.from
    view.dispatch({
      changes: { from: pos, insert: text },
      selection: { anchor: pos + text.length },
    })
    view.focus()
  }

  function insertCodeBlock(): void {
    if (!editorView) return
    const view = editorView
    const tr = view.state.changeByRange((range) => {
      const text = view.state.doc.sliceString(range.from, range.to)
      const insert = `\`\`\`\n${text}\n\`\`\``
      return {
        changes: [{ from: range.from, to: range.to, insert }],
        range: EditorSelection.cursor(range.from + 4 + text.length),
      }
    })
    view.dispatch(view.state.update(tr))
    view.focus()
  }

  function closeContextMenu(): void {
    contextMenu = null
    showTemplatePicker = false
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
    await openTab(path)
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

  function clearCurrentNoteView(): void {
    currentNote = null
    source = ''
    html = ''
    backlinks = []
    noteTags = []
    outline = []
  }

  async function deleteNote(name: string): Promise<void> {
    await DeleteNote(name)
    openTabs = openTabs.filter((p) => p !== name)
    if (currentNote === name) {
      const next = openTabs[0]
      if (next) await openTab(next)
      else clearCurrentNoteView()
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
      openTabs = openTabs.map((p) => (p === oldPath || p.startsWith(oldPath + '/') ? newPath + p.slice(oldPath.length) : p))
      if (currentNote === oldPath || currentNote?.startsWith(oldPath + '/')) {
        currentNote = newPath + currentNote!.slice(oldPath.length)
      }
    } else {
      await RenameNote(oldPath, newPath)
      openTabs = openTabs.map((p) => (p === oldPath ? newPath : p))
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
      openTabs = openTabs.map((p) => (p === path || p.startsWith(path + '/') ? newPath + p.slice(path.length) : p))
      if (currentNote === path || currentNote?.startsWith(path + '/')) {
        currentNote = newPath + currentNote!.slice(path.length)
      }
    } else {
      await RenameNote(path, newPath)
      openTabs = openTabs.map((p) => (p === path ? newPath : p))
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
    await openTab(name)
  }

  async function changeVault(): Promise<void> {
    vaultPath = await SelectVault()
    openTabs = []
    clearCurrentNoteView()
    searchQuery = ''
    activeTag = null
    await refreshList()
    await loadTemplateList()
  }

  async function onVaultChanged(): Promise<void> {
    await refreshList()
    await loadTemplateList()
    openTabs = openTabs.filter((p) => notes.includes(p))
    if (currentNote && !notes.includes(currentNote)) {
      clearCurrentNoteView()
    } else if (currentNote) {
      backlinks = await GetBacklinks(currentNote)
      noteTags = await GetTags(currentNote)
    }
  }

  onMount(async () => {
    vaultPath = await GetVaultPath()
    templatesFolder = await GetTemplatesFolder()
    dailyNoteFolder = await GetDailyNoteFolder()
    dailyNoteTemplate = await GetDailyNoteTemplate()
    await loadTemplateList()
    await refreshList()
    EventsOn('vault:changed', onVaultChanged)
    EventsOn('image:dropped', (x: number, y: number, relPath: string) => {
      if (!editorView) return
      const pos = editorView.posAtCoords({ x, y }) ?? editorView.state.selection.main.from
      const insert = `![](${relPath})`
      editorView.dispatch({
        changes: { from: pos, insert },
        selection: { anchor: pos + insert.length },
      })
    })
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
      {#if currentNote && !showGraph && viewMode !== 'editor'}
        <button on:click={() => (showOutline = !showOutline)} title="アウトライン" class:active={showOutline}>
          <AlignLeft size={16} />
        </button>
      {/if}
      <button on:click={openDailyNote} title="デイリーノート (Ctrl+D)"><Calendar size={16} /></button>
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
                on:click={() => openTab(path)}
                on:dblclick={() => startRename(path)}
              >{path}</span>
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
            onSelect={openTab}
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

  {#if openTabs.length && !showGraph}
    <div class="tab-bar">
      {#each openTabs as path (path)}
        <div
          class="tab"
          class:active={path === currentNote}
          on:click={() => openTab(path)}
        >
          <FileText size={13} />
          <span class="tab-label">{basename(path)}</span>
          <button class="tab-close" on:click={(e) => closeTab(path, e)}><X size={12} /></button>
        </div>
      {/each}
    </div>
  {/if}

  {#if showGraph}
    <div class="graph-view">
      <GraphView {notes} edges={graphEdges} {currentNote} on:select={onGraphSelect} />
    </div>
  {:else if currentNote}
    <div class="editor" class:full={viewMode === 'editor'} class:hidden={viewMode === 'preview'}>
      <div class="editor-toolbar">
        <button on:click={() => editorView && undo(editorView)} title="元に戻す"><Undo2 size={15} /></button>
        <button on:click={() => editorView && redo(editorView)} title="やり直す"><Redo2 size={15} /></button>
        <span class="toolbar-divider"></span>
        <button on:click={() => prefixLines('## ')} title="見出し"><Heading2 size={15} /></button>
        <button on:click={() => wrapSelection('**')} title="太字"><Bold size={15} /></button>
        <button on:click={() => wrapSelection('*')} title="斜体"><Italic size={15} /></button>
        <button on:click={() => wrapSelection('<u>', '</u>')} title="下線"><Underline size={15} /></button>
        <button on:click={() => wrapSelection('~~')} title="取り消し線"><Strikethrough size={15} /></button>
        <span class="toolbar-divider"></span>
        <button on:click={() => wrapSelection('`')} title="インラインコード"><Code size={15} /></button>
        <button on:click={insertCodeBlock} title="コードブロック"><Code2 size={15} /></button>
        <button on:click={insertLink} title="リンク"><LinkIcon size={15} /></button>
        <button on:click={insertImageFromFile} title="画像を挿入"><Image size={15} /></button>
        <span class="toolbar-divider"></span>
        <button on:click={() => prefixLines('- ')} title="箇条書き"><List size={15} /></button>
        <button on:click={() => prefixLines('1. ')} title="番号付きリスト"><ListOrdered size={15} /></button>
        <button on:click={() => prefixLines('- [ ] ')} title="チェックリスト"><ListChecks size={15} /></button>
        <button on:click={() => prefixLines('> ')} title="引用"><Quote size={15} /></button>
        <span class="toolbar-divider"></span>
        <button on:click={() => insertAtCursor('\n---\n')} title="水平線"><Minus size={15} /></button>
        <button on:click={() => insertAtCursor('\n| 見出し1 | 見出し2 |\n| --- | --- |\n|  |  |\n')} title="表"><Table size={15} /></button>
        {#if templateList.length}
          <span class="toolbar-divider"></span>
          <div class="template-picker">
            <button on:click|stopPropagation={() => (showTemplatePicker = !showTemplatePicker)} title="テンプレートを挿入">
              <FileStack size={15} />
            </button>
            {#if showTemplatePicker}
              <div class="template-picker-menu">
                {#each templateList as name}
                  <button on:click={() => insertTemplate(name)}>{name}</button>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
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
      <div bind:this={previewContentEl} on:click={onPreviewClick}>{@html html}</div>
      {#if backlinks.length}
        <div class="backlinks">
          <div class="backlinks-title"><Link2 size={13} /> バックリンク</div>
          <ul>
            {#each backlinks as name}
              <li><span class="note-name" on:click={() => openTab(name)}>{name}</span></li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
    {#if showOutline && outline.length}
      <div class="outline-panel">
        <div class="outline-title"><AlignLeft size={13} /> アウトライン</div>
        <ul>
          {#each outline as item}
            <li style="padding-left: {(item.level - 1) * 0.8}rem">
              <span on:click={() => jumpToHeading(item)}>{item.text}</span>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
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
    <button class="settings-trigger" title="設定" on:click={() => (showSettings = true)}>
      <Settings size={14} />
    </button>
    <div class="bottombar-right">
      {#if saveStatus}
        <span class="save-status"><Check size={13} /> {saveStatus}</span>
      {/if}
      {#if currentNote}
        <span class="note-stats">
          {charCount}文字
          <span class="stat-sep">·</span>
          {lineCount}行
          {#if outline.length}
            <span class="stat-sep">·</span>
            {outline.length}見出し
          {/if}
          {#if readingMins}
            <span class="stat-sep">·</span>
            {readingMins}分
          {/if}
        </span>
      {/if}
      <button on:click={toggleTerminal} title="ターミナル" class:active={showTerminal}>
        <TerminalSquare size={14} />
      </button>
    </div>
  </footer>

  {#if showSettings}
    <div class="modal-overlay" on:click={() => (showSettings = false)}>
      <div class="settings-modal" on:click|stopPropagation>
        <button class="modal-close" on:click={() => (showSettings = false)}><X size={18} /></button>
        <div class="settings-modal-body">
          <nav class="settings-nav">
            <button
              class:active={settingsCategory === 'general'}
              on:click={() => (settingsCategory = 'general')}
            >全般</button>
            <button
              class:active={settingsCategory === 'appearance'}
              on:click={() => (settingsCategory = 'appearance')}
            >外観</button>
            <button
              class:active={settingsCategory === 'templates'}
              on:click={() => (settingsCategory = 'templates')}
            >テンプレート</button>
          </nav>
          <div class="settings-content">
            {#if settingsCategory === 'general'}
              <h3>全般</h3>
              <div class="settings-row">
                <FolderOpen size={14} />
                <span class="vault-path" title={vaultPath}>{vaultPath}</span>
                <button on:click={changeVault}>変更</button>
              </div>
            {:else if settingsCategory === 'appearance'}
              <h3>外観</h3>
              <div class="settings-row">
                <button class="settings-action" on:click={toggleTheme}>
                  {#if theme === 'dark'}<Sun size={14} />{:else}<Moon size={14} />{/if}
                  テーマ：{theme === 'dark' ? 'ダーク' : 'ライト'}
                </button>
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
                    await loadTemplateList()
                  }}
                />
              </div>
              <div class="settings-row">
                <span>デイリーノートフォルダ</span>
                <input
                  type="text"
                  bind:value={dailyNoteFolder}
                  on:change={() => SetDailyNoteFolder(dailyNoteFolder)}
                />
              </div>
              <div class="settings-row">
                <span>デイリーノートのテンプレート</span>
                <select
                  bind:value={dailyNoteTemplate}
                  on:change={() => SetDailyNoteTemplate(dailyNoteTemplate)}
                >
                  <option value="">なし</option>
                  {#each templateList as name}
                    <option value={name}>{name}</option>
                  {/each}
                </select>
              </div>
              <p class="settings-hint">テンプレート内の <code>{'{{date}}'}</code> は挿入時に日付へ置き換わる</p>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {/if}

  {#if showQuickSwitcher}
    <div class="modal-overlay qs-overlay" on:click={closeQuickSwitcher}>
      <div class="quick-switcher" on:click|stopPropagation>
        <input
          bind:this={qsInputEl}
          bind:value={qsQuery}
          on:keydown={onQsKeydown}
          class="qs-input"
          placeholder="ノートを開く、または新規作成..."
        />
        <ul class="qs-list">
          {#each qsResults as path, i}
            <li class:active={i === qsIndex} on:click={() => qsSelect(path)}>
              <FileText size={14} />{path}
            </li>
          {/each}
          {#if qsShowCreate}
            <li class:active={qsIndex === qsResults.length} on:click={() => qsSelect(qsQuery.trim())}>
              <FilePlus size={14} />新規ノートを作成: "{qsQuery.trim()}"
            </li>
          {/if}
        </ul>
      </div>
    </div>
  {/if}

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
    grid-template-rows: auto auto 1fr auto auto;
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

  .topbar-right button.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
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
    grid-row: 3 / 5;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .tab-bar {
    grid-column: 2 / 4;
    grid-row: 2;
    display: flex;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
    background: var(--bg-secondary);
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.4rem 0.5rem;
    border-right: 1px solid var(--border);
    font-size: 0.8rem;
    color: var(--text-dim);
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .tab.active {
    background: var(--bg);
    color: var(--text);
  }

  .tab-label {
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tab-close {
    display: flex;
    align-items: center;
    border: none;
    background: none;
    color: inherit;
    opacity: 0.5;
    cursor: pointer;
    border-radius: 3px;
  }

  .tab-close:hover {
    opacity: 1;
    background: var(--bg-hover);
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
    grid-row: 3;
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
    flex-wrap: wrap;
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

  .toolbar-divider {
    width: 1px;
    height: 1.2rem;
    background: var(--border);
    margin: 0 0.2rem;
  }

  .template-picker {
    position: relative;
  }

  .template-picker-menu {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 10;
    display: flex;
    flex-direction: column;
    min-width: 140px;
    margin-top: 0.2rem;
    padding: 0.3rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  }

  .template-picker-menu button {
    display: block;
    width: 100%;
    padding: 0.35rem 0.5rem;
    text-align: left;
    background: none;
    border: none;
    color: var(--text);
    font-size: 0.8rem;
    border-radius: 3px;
    cursor: pointer;
  }

  .template-picker-menu button:hover {
    background: var(--bg-hover);
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
    grid-row: 3;
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

  .outline-panel {
    position: fixed;
    top: calc(2.5rem + 1px);
    right: 0.6rem;
    width: 220px;
    max-height: 60vh;
    overflow-y: auto;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    padding: 0.5rem;
    z-index: 8;
  }

  .outline-title {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.75rem;
    opacity: 0.6;
    margin-bottom: 0.4rem;
  }

  .outline-panel ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .outline-panel li {
    padding: 0.2rem 0;
  }

  .outline-panel li span {
    cursor: pointer;
    font-size: 0.8rem;
    color: var(--text-dim);
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .outline-panel li span:hover {
    color: var(--accent);
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
    grid-row: 3;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }

  .graph-view {
    grid-column: 2 / 4;
    grid-row: 3;
    min-height: 0;
    overflow: hidden;
  }

  .bottombar {
    grid-column: 1 / 4;
    grid-row: 5;
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
    grid-row: 4;
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

  .settings-trigger {
    display: flex;
    align-items: center;
    border: none;
    background: none;
    color: inherit;
    cursor: pointer;
    opacity: 0.7;
    padding: 0.2rem;
  }

  .settings-trigger:hover {
    opacity: 1;
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

  .qs-overlay {
    align-items: flex-start;
    padding-top: 12vh;
  }

  .quick-switcher {
    width: 560px;
    max-width: 90vw;
    max-height: 60vh;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .qs-input {
    border: none;
    border-bottom: 1px solid var(--border);
    background: none;
    color: var(--text);
    padding: 0.8rem 1rem;
    font-size: 1rem;
  }

  .qs-input:focus {
    outline: none;
  }

  .qs-list {
    list-style: none;
    margin: 0;
    padding: 0.3rem;
    overflow-y: auto;
  }

  .qs-list li {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.6rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .qs-list li.active {
    background: var(--accent-hover);
  }

  .settings-modal {
    position: relative;
    width: 640px;
    max-width: 90vw;
    height: 440px;
    max-height: 80vh;
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

  .settings-row .vault-path {
    flex: 1;
    max-width: none;
  }

  .settings-row > span {
    flex: 1;
  }

  .settings-row input[type='text'],
  .settings-row select {
    padding: 0.3rem 0.5rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text);
    font-size: 0.85rem;
  }

  .settings-hint {
    padding: 0.3rem;
    font-size: 0.75rem;
    opacity: 0.6;
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

  .bottombar-right {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    opacity: 0.7;
  }

  .note-stats {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.75rem;
  }

  .stat-sep {
    opacity: 0.4;
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
