<script lang="ts">
  import { onMount, tick } from 'svelte'
  import renderMathInElement from 'katex/contrib/auto-render'
  import 'katex/dist/katex.min.css'
  import mermaid from 'mermaid'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import { EditorView, lineNumbers, highlightSpecialChars, drawSelection, dropCursor, highlightActiveLine, keymap } from '@codemirror/view'
  import { EditorState, EditorSelection, Prec } from '@codemirror/state'
  import { history, defaultKeymap, historyKeymap, undo, redo } from '@codemirror/commands'
  import { vim } from '@replit/codemirror-vim'
  import { markdown } from '@codemirror/lang-markdown'
  import { languages } from '@codemirror/language-data'
  import { oneDark } from '@codemirror/theme-one-dark'
  import { HighlightStyle, syntaxHighlighting, defaultHighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language'
  import { tags } from '@lezer/highlight'
  import TreeItem from './TreeItem.svelte'
  import type { TreeNode } from './TreeItem.svelte'
  import GraphView from './GraphView.svelte'
  import TerminalPanel from './Terminal.svelte'
  import Kanban from './Kanban.svelte'
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
    SearchWithSnippets,
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
    WriteTerminal,
    StartTerminal,
    PrepareRunFile,
    ExportHTML,
    ExportPDF,
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
  let searchSeq = 0
  let searchBusy = false
  let vaultPath = ''
  let templatesFolder = ''
  let dailyNoteFolder = ''
  let dailyNoteTemplate = ''
  let templateList: string[] = []
  let showTemplatePicker = false
  let searchInputEl: HTMLInputElement
  const searchOperators = ['tag:', 'file:', 'path:', 'line:', 'section:']
  let searchHits: { path: string; snippets: string[] }[] = []
  const hasOperator = (q: string) => searchOperators.some((op) => q.includes(op))

  function highlightQuery(text: string, query: string): string {
    const safe = text.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!))
    const esc = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    return safe.replace(new RegExp(esc, 'gi'), '<mark>$&</mark>')
  }

  let saveStatus = ''
  let saveStatusTimer: ReturnType<typeof setTimeout>
  let lastSelfSavedContent: Map<string, string> = new Map()
  let toast = ''
  let toastTimer: ReturnType<typeof setTimeout>

  function showToast(msg: string) {
    toast = msg
    clearTimeout(toastTimer)
    toastTimer = setTimeout(() => (toast = ''), 4000)
  }
  let contextMenu: { x: number; y: number; type: 'empty' | 'note' | 'folder'; path?: string } | null = null
  let theme: 'dark' | 'light' = (localStorage.getItem('knote-theme') as 'dark' | 'light' | null) ?? 'dark'
  let vimMode: boolean = localStorage.getItem('knote-vim') === 'true'
  let activeTag: string | null = null
  let noteTags: string[] = []
  let viewMode: 'split' | 'editor' | 'preview' = 'split'
  let showSettings = false
  let showKanban = false

  function isKanbanNote(src: string): boolean {
    const fm = src.match(/^---\r?\n([\s\S]*?)\r?\n---/)
    if (!fm) return false
    return /^\s*kanban:\s*true\s*$/m.test(fm[1])
  }

  function onKanbanChange(e: CustomEvent<string>) {
    source = e.detail
    SaveNote(currentNote!, source)
    lastSelfSavedContent.set(currentNote!, source)
  }
  let settingsCategory: 'general' | 'appearance' | 'templates' = 'general'
  let showQuickSwitcher = false
  let qsQuery = ''
  let qsIndex = 0
  let qsInputEl: HTMLInputElement

  type PaletteItem =
    | { kind: 'cmd'; label: string; action: () => void }
    | { kind: 'note'; path: string }
    | { kind: 'create'; path: string }

  async function doExportHTML() {
    if (!currentNote) return
    try {
      const path = await ExportHTML(currentNote)
      showToast(`HTML保存: ${path}`)
    } catch (e) {
      showToast(`エクスポート失敗: ${e}`)
    }
  }

  async function doExportPDF() {
    if (!currentNote) return
    try {
      const path = await ExportPDF(currentNote)
      showToast(`PDF保存: ${path}`)
    } catch (e) {
      showToast(`エクスポート失敗: ${e}`)
    }
  }

  $: paletteCommands = [
    { label: '新規ノートを作成', action: () => createNoteAt('') },
    { label: 'デイリーノートを開く (Ctrl+D)', action: openDailyNote },
    { label: `テーマを${theme === 'dark' ? 'ライト' : 'ダーク'}に切り替え`, action: toggleTheme },
    { label: `Vimモードを${vimMode ? '無効化' : '有効化'}`, action: toggleVim },
    { label: `表示: ${viewMode === 'split' ? 'エディタのみ' : viewMode === 'editor' ? 'プレビューのみ' : '分割'}に切り替え`, action: () => { viewMode = viewMode === 'split' ? 'editor' : viewMode === 'editor' ? 'preview' : 'split' } },
    { label: `ターミナルを${showTerminal ? '閉じる' : '開く'}`, action: () => { showTerminal = !showTerminal } },
    { label: 'グラフビューを表示', action: () => { showGraph = true } },
    { label: '設定を開く', action: () => { showSettings = true } },
    ...(currentNote ? [{ label: `「${currentNote}」を閉じる`, action: () => { if (currentNote) closeTab(currentNote) } }] : []),
    { label: 'テーブルを整形 (Markdown)', action: () => { if (editorView) formatCurrentTable(editorView) } },
    ...(currentNote ? [{ label: 'HTMLとしてエクスポート', action: doExportHTML }] : []),
    ...(currentNote ? [{ label: 'PDFとしてエクスポート', action: doExportPDF }] : []),
  ]

  $: paletteItems = (() => {
    const q = qsQuery.trim()
    const ql = q.toLowerCase()
    const items: PaletteItem[] = []
    if (q.startsWith('>')) {
      const cmdQ = ql.slice(1).trim()
      for (const cmd of paletteCommands) {
        if (!cmdQ || cmd.label.toLowerCase().includes(cmdQ)) {
          items.push({ kind: 'cmd', label: cmd.label, action: cmd.action })
        }
      }
    } else {
      const noteResults = q ? notes.filter((n) => n.toLowerCase().includes(ql)) : notes.slice(0, 20)
      for (const path of noteResults) items.push({ kind: 'note', path })
      if (q && !notes.includes(q)) items.push({ kind: 'create', path: q })
    }
    return items
  })()

  $: qsTotal = paletteItems.length
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

  async function executeItem(item: PaletteItem): Promise<void> {
    closeQuickSwitcher()
    if (item.kind === 'cmd') {
      item.action()
    } else if (item.kind === 'note') {
      await openTab(item.path)
    } else {
      if (!notes.includes(item.path)) {
        await CreateNote(item.path)
        await refreshList()
      }
      await openTab(item.path)
    }
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
      const item = paletteItems[qsIndex]
      if (item) executeItem(item)
    }
  }

  function toggleTheme(): void {
    theme = theme === 'dark' ? 'light' : 'dark'
    localStorage.setItem('knote-theme', theme)
  }

  function toggleVim(): void {
    vimMode = !vimMode
    localStorage.setItem('knote-vim', String(vimMode))
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
    if (searchBusy) return
    searchBusy = true
    const seq = ++searchSeq
    const q = searchQuery.trim()
    try {
      let hits: { path: string; snippets: string[] }[] = []
      let paths: string[] = []
      if (q) {
        activeTag = null
        if (hasOperator(q)) {
          paths = await SearchNotes(q)
        } else {
          hits = await SearchWithSnippets(q)
          paths = hits.map((h) => h.path)
        }
      } else if (activeTag) {
        paths = await SearchByTag(activeTag)
      } else {
        paths = notes
      }
      if (seq === searchSeq) {
        searchHits = hits
        visibleNotes = paths
      }
    } finally {
      searchBusy = false
      // if query changed while we were running, re-run once
      if (seq !== searchSeq) runSearch()
    }
  }

  function onSearchInput(): void {
    ++searchSeq
    clearTimeout(searchTimer)
    searchTimer = setTimeout(runSearch, 350)
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
    showKanban = isKanbanNote(source)
    await render()
    backlinks = await GetBacklinks(name)
    noteTags = await GetTags(name)
  }

  async function openTab(path: string): Promise<void> {
    if (currentNote) {
      clearTimeout(saveTimer)
      lastSelfSavedContent.set(currentNote, source)
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
    await tick()
    await updateOutline()
    applyCodeBlockButtons()
    applyMath()
    await applyMermaid()
  }

  function applyMath(): void {
    if (!previewContentEl) return
    renderMathInElement(previewContentEl, {
      delimiters: [
        { left: '$$', right: '$$', display: true },
        { left: '$', right: '$', display: false },
      ],
      throwOnError: false,
    })
  }

  let mermaidInitialized = false

  async function applyMermaid(): Promise<void> {
    if (!previewContentEl) return
    if (!mermaidInitialized) {
      mermaid.initialize({ startOnLoad: false, theme: 'dark' })
      mermaidInitialized = true
    }
    const blocks = Array.from(
      previewContentEl.querySelectorAll('code.language-mermaid')
    ) as HTMLElement[]
    for (const code of blocks) {
      const pre = code.parentElement
      if (!pre || pre.dataset.mermaidRendered) continue
      const source = code.textContent ?? ''
      try {
        const id = 'mermaid-' + Math.random().toString(36).slice(2)
        const { svg } = await mermaid.render(id, source)
        const wrapper = document.createElement('div')
        wrapper.className = 'mermaid-diagram'
        wrapper.innerHTML = svg
        pre.replaceWith(wrapper)
      } catch {
        pre.dataset.mermaidRendered = 'error'
      }
    }
  }

  function applyCodeBlockButtons(): void {
    if (!previewContentEl) return
    for (const pre of Array.from(previewContentEl.querySelectorAll('pre')) as HTMLElement[]) {
      const code = pre.querySelector('code')
      if (!code) continue
      pre.style.position = 'relative'
      const btn = document.createElement('button')
      btn.className = 'code-run-btn'
      btn.textContent = '▶ run'
      btn.title = 'ターミナルで実行'
      btn.addEventListener('click', async (e) => {
        e.stopPropagation()
        const text = (code.textContent || '').trimEnd()
        if (!text) return
        const lang = (code.className || '').replace('language-', '').split(' ')[0]
        showTerminal = true
        await StartTerminal()
        const cmd = await PrepareRunFile(lang, text)
        WriteTerminal(cmd + '\n')
      })
      pre.appendChild(btn)
    }
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
    saveTimer = setTimeout(() => {
      lastSelfSavedContent.set(currentNote!, source)
      SaveNote(currentNote!, source)
    }, 400)
  }

  async function forceSave(): Promise<void> {
    if (!currentNote) return
    clearTimeout(saveTimer)
    lastSelfSavedContent.set(currentNote, source)
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
  let previewEl: HTMLDivElement
  let lastEditorScroll = 0
  let lastPreviewScroll = 0

  function onPreviewScroll(): void {
    if (!editorView || !previewEl) return
    if (Date.now() - lastEditorScroll < 120) return
    lastPreviewScroll = Date.now()
    const ratio = previewEl.scrollTop / Math.max(1, previewEl.scrollHeight - previewEl.clientHeight)
    editorView.scrollDOM.scrollTop = ratio * Math.max(0, editorView.scrollDOM.scrollHeight - editorView.scrollDOM.clientHeight)
  }

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

  function isTableLine(text: string): boolean {
    return text.trimStart().startsWith('|')
  }

  function isSeparatorLine(text: string): boolean {
    return /^\s*\|[-: |]+\|\s*$/.test(text)
  }

  function getTableRange(state: EditorState): { from: number; to: number; lines: string[] } | null {
    const line = state.doc.lineAt(state.selection.main.head)
    if (!isTableLine(line.text)) return null
    let startNum = line.number
    while (startNum > 1 && isTableLine(state.doc.line(startNum - 1).text)) startNum--
    let endNum = line.number
    while (endNum < state.doc.lines && isTableLine(state.doc.line(endNum + 1).text)) endNum++
    const lines: string[] = []
    for (let i = startNum; i <= endNum; i++) lines.push(state.doc.line(i).text)
    return { from: state.doc.line(startNum).from, to: state.doc.line(endNum).to, lines }
  }

  function formatTableText(lines: string[]): string {
    const rows = lines.map((l) => l.split('|').slice(1, -1).map((c) => c.trim()))
    const colCount = Math.max(...rows.map((r) => r.length))
    const widths: number[] = Array(colCount).fill(3)
    for (const row of rows) {
      for (let i = 0; i < row.length; i++) {
        if (!/^[-: ]+$/.test(row[i])) widths[i] = Math.max(widths[i], row[i].length)
      }
    }
    return lines.map((line, ri) => {
      if (isSeparatorLine(line)) return '| ' + widths.map((w) => '-'.repeat(w)).join(' | ') + ' |'
      return '| ' + widths.map((w, i) => (rows[ri][i] ?? '').padEnd(w)).join(' | ') + ' |'
    }).join('\n')
  }

  function formatCurrentTable(view: EditorView): boolean {
    const range = getTableRange(view.state)
    if (!range) return false
    const formatted = formatTableText(range.lines)
    const cursorOffset = view.state.selection.main.head - range.from
    view.dispatch({ changes: { from: range.from, to: range.to, insert: formatted } })
    const newPos = Math.min(range.from + cursorOffset, range.from + formatted.length)
    view.dispatch({ selection: { anchor: newPos } })
    return true
  }

  function tableNextCell(view: EditorView): boolean {
    const { state } = view
    const pos = state.selection.main.head
    const line = state.doc.lineAt(pos)
    if (!isTableLine(line.text) || isSeparatorLine(line.text)) return false
    const afterCursor = line.text.slice(pos - line.from)
    const nextPipe = afterCursor.indexOf('|', 1)
    if (nextPipe !== -1) {
      const newPos = pos + nextPipe + 1
      const spaces = (line.text.slice(newPos - line.from).match(/^ */) ?? [''])[0].length
      view.dispatch({ selection: { anchor: newPos + spaces } })
      return true
    }
    // end of row: go to next non-separator row or insert
    let nextNum = line.number + 1
    while (nextNum <= state.doc.lines) {
      const nl = state.doc.line(nextNum)
      if (!isTableLine(nl.text)) break
      if (!isSeparatorLine(nl.text)) {
        const firstPipe = nl.text.indexOf('|') + 1
        const spaces = (nl.text.slice(firstPipe).match(/^ */) ?? [''])[0].length
        view.dispatch({ selection: { anchor: nl.from + firstPipe + spaces } })
        return true
      }
      nextNum++
    }
    return tableInsertRow(view)
  }

  function tablePrevCell(view: EditorView): boolean {
    const { state } = view
    const pos = state.selection.main.head
    const line = state.doc.lineAt(pos)
    if (!isTableLine(line.text) || isSeparatorLine(line.text)) return false
    const beforeCursor = line.text.slice(0, pos - line.from)
    const pipes = [...beforeCursor.matchAll(/\|/g)].map((m) => m.index!)
    if (pipes.length >= 2) {
      const prevPipe = pipes[pipes.length - 2]
      const cellStart = line.from + prevPipe + 1
      const spaces = (line.text.slice(prevPipe + 1).match(/^ */) ?? [''])[0].length
      view.dispatch({ selection: { anchor: cellStart + spaces } })
      return true
    }
    // beginning of row: go to last cell of previous non-separator row
    let prevNum = line.number - 1
    while (prevNum >= 1) {
      const pl = state.doc.line(prevNum)
      if (!isTableLine(pl.text)) break
      if (!isSeparatorLine(pl.text)) {
        const lastPipe = pl.text.lastIndexOf('|', pl.text.length - 2)
        if (lastPipe !== -1) {
          const spaces = (pl.text.slice(lastPipe + 1).match(/^ */) ?? [''])[0].length
          view.dispatch({ selection: { anchor: pl.from + lastPipe + 1 + spaces } })
          return true
        }
      }
      prevNum--
    }
    return false
  }

  function tableInsertRow(view: EditorView): boolean {
    const { state } = view
    const line = state.doc.lineAt(state.selection.main.head)
    if (!isTableLine(line.text) || isSeparatorLine(line.text)) return false
    const colCount = (line.text.match(/\|/g) ?? []).length - 1
    const newRow = '\n|' + ' |'.repeat(colCount)
    view.dispatch({
      changes: { from: line.to, insert: newRow },
      selection: { anchor: line.to + 2 },
    })
    return true
  }

  function tableKeymap(): Extension {
    return Prec.highest(keymap.of([
      { key: 'Tab', run: tableNextCell },
      { key: 'Shift-Tab', run: tablePrevCell },
      { key: 'Enter', run: tableInsertRow },
    ]))
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
          ...(vimMode ? [vim()] : [tableKeymap(), keymap.of([...defaultKeymap, ...historyKeymap])]),
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
    view.scrollDOM.addEventListener('scroll', () => {
      if (!previewEl) return
      if (Date.now() - lastPreviewScroll < 120) return
      lastEditorScroll = Date.now()
      const ratio = view.scrollDOM.scrollTop / Math.max(1, view.scrollDOM.scrollHeight - view.scrollDOM.clientHeight)
      previewEl.scrollTop = ratio * Math.max(0, previewEl.scrollHeight - previewEl.clientHeight)
    })
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
    EventsOn('vault:note-changed', async (noteName: string) => {
      if (noteName !== currentNote) return
      const fresh = await ReadNote(noteName)
      if (fresh === lastSelfSavedContent.get(noteName)) {
        lastSelfSavedContent.delete(noteName)
        return
      }
      if (fresh === source) return
      source = fresh
      if (editorView) {
        editorView.dispatch({
          changes: { from: 0, to: editorView.state.doc.length, insert: fresh },
        })
      }
      await render()
      showToast(`「${noteName}」が外部で変更されたため再読み込みしました`)
    })
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
      {#if currentNote && !showGraph && !showKanban}
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
      {#if currentNote && !showGraph && !showKanban && viewMode !== 'editor'}
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
          <li
            class="search-result"
            class:active={path === currentNote}
            on:click={() => openTab(path)}
            on:contextmenu={(e) => onNoteContextMenu(e, path)}
          >
            <span class="note-name"><FileText size={13} />{path}</span>
            {#if hit}
              <ul class="snippets">
                {#each hit.snippets as snippet}
                  <li>{@html highlightQuery(snippet, searchQuery.trim())}</li>
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
  {#if currentNote && !showGraph && !showKanban && viewMode === 'split'}
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
  {:else if currentNote && showKanban}
    <div class="kanban-area">
      <Kanban {source} on:change={onKanbanChange} />
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
      {#key currentNote + theme + vimMode}
        <div class="editor-mount" use:initEditor></div>
      {/key}
    </div>
    <div class="preview" class:full={viewMode === 'preview'} class:hidden={viewMode === 'editor'} class:no-scroll={showQuickSwitcher || showSettings} bind:this={previewEl} on:scroll={onPreviewScroll}>
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
              <div class="settings-row">
                <button class="settings-action" on:click={toggleVim}>
                  Vimモード：{vimMode ? 'オン' : 'オフ'}
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
          placeholder={qsQuery.startsWith('>') ? 'コマンドを検索...' : 'ノートを開く... (「>」でコマンド)'}
        />
        <ul class="qs-list">
          {#each paletteItems as item, i}
            <li class:active={i === qsIndex} on:click={() => executeItem(item)}>
              {#if item.kind === 'cmd'}
                <Code2 size={14} /><span class="qs-label">{item.label}</span>
              {:else if item.kind === 'note'}
                <FileText size={14} />{item.path}
              {:else}
                <FilePlus size={14} />新規ノートを作成: "{item.path}"
              {/if}
            </li>
          {/each}
          {#if paletteItems.length === 0}
            <li class="qs-empty">一致する項目がありません</li>
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

{#if toast}
  <div class="toast">{toast}</div>
{/if}

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

  .resize-handle-v::after {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 3px;
    height: 40px;
    border-radius: 2px;
    background: transparent;
    transition: background 0.15s;
  }

  .resize-handle-v:hover::after {
    background: var(--accent);
  }

  .resize-handle-h {
    top: -3px;
    left: 0;
    right: 0;
    height: 5px;
    cursor: row-resize;
    z-index: 6;
  }

  .resize-handle-h::after {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 40px;
    height: 3px;
    border-radius: 2px;
    background: transparent;
    transition: background 0.15s;
  }

  .resize-handle-h:hover::after {
    background: var(--accent);
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

  .preview.no-scroll {
    overflow: hidden;
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

  .preview :global(.code-run-btn) {
    position: absolute;
    top: 0.4rem;
    right: 0.4rem;
    padding: 0.15rem 0.5rem;
    background: rgba(0, 0, 0, 0.4);
    color: #ccc;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 3px;
    font-size: 0.7rem;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s;
    font-family: inherit;
  }

  .preview :global(pre:hover .code-run-btn) {
    opacity: 1;
  }

  .preview :global(.code-run-btn:hover) {
    background: rgba(80, 80, 80, 0.6);
    color: #fff;
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

  .preview :global(.mermaid-diagram) {
    display: flex;
    justify-content: center;
    margin: 1rem 0;
    overflow-x: auto;
  }

  .preview :global(.mermaid-diagram svg) {
    max-width: 100%;
    height: auto;
  }

  .preview :global(.katex-display) {
    overflow-x: auto;
    padding: 0.5rem 0;
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

  .kanban-area {
    grid-column: 2 / 4;
    grid-row: 3;
    min-height: 0;
    display: flex;
    flex-direction: column;
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
    justify-content: flex-start;
    gap: 0.5rem;
    padding: 0.5rem 0.6rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .qs-list li.active {
    background: var(--accent-hover);
  }

  .qs-list li:hover {
    background: var(--bg-hover);
  }

  .qs-list li.active:hover {
    background: var(--accent-hover);
  }

  .qs-empty {
    color: var(--text-muted);
    font-size: 0.85rem;
    justify-content: center;
    cursor: default;
  }

  .qs-label {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

  :global(.toast) {
    position: fixed;
    bottom: 2.5rem;
    right: 1.5rem;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.5rem 1rem;
    font-size: 0.82rem;
    color: var(--text);
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.4);
    z-index: 9999;
    max-width: 400px;
    word-break: break-all;
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
