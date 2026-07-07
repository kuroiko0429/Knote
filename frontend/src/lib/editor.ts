import { EditorView, lineNumbers, keymap, ViewPlugin, Decoration } from '@codemirror/view'
import { EditorState, Prec, RangeSetBuilder, StateEffect, StateField, type Extension } from '@codemirror/state'
import { autocompletion, type CompletionContext } from '@codemirror/autocomplete'
import { HighlightStyle } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { Vim } from '@replit/codemirror-vim'

// StateEffect/Field for vim mode — lets relativeLineNumbers read mode from CM state
export const vimModeEffect = StateEffect.define<string>()
export const vimModeField = StateField.define<string>({
  create: () => 'normal',
  update: (val, tr) => {
    for (const e of tr.effects) if (e.is(vimModeEffect)) return e.value
    return val
  },
})

let _vimBindingsSetup = false

export interface VimActions {
  forceSave: () => void
  closeTab: (path: string) => void
  openTab: (path: string) => void
  getCurrentNote: () => string | null
  getOpenTabs: () => string[]
  getNotes: () => string[]
  focusSearchInput: () => void
}

export function setupVimGlobal(actions: VimActions): void {
  if (_vimBindingsSetup) return
  _vimBindingsSetup = true

  const { forceSave, closeTab, openTab, getCurrentNote, getOpenTabs, getNotes, focusSearchInput } = actions

  // insert: jk → Esc
  Vim.map('jk', '<Esc>', 'insert')

  // Space leader mappings in normal mode
  Vim.defineEx('knotesave', '', () => forceSave())
  Vim.defineEx('knoteclose', '', () => { const currentNote = getCurrentNote(); if (currentNote) closeTab(currentNote) })
  Vim.defineEx('knotesearch', '', () => { setTimeout(() => focusSearchInput(), 50) })
  Vim.defineEx('knotenext', '', () => {
    const currentNote = getCurrentNote()
    if (!currentNote) return
    const openTabs = getOpenTabs()
    const i = openTabs.indexOf(currentNote)
    const next = openTabs[i + 1] ?? openTabs[0]
    if (next) openTab(next)
  })
  Vim.defineEx('knoteprev', '', () => {
    const currentNote = getCurrentNote()
    if (!currentNote) return
    const openTabs = getOpenTabs()
    const i = openTabs.indexOf(currentNote)
    const prev = openTabs[i - 1] ?? openTabs[openTabs.length - 1]
    if (prev) openTab(prev)
  })

  // Standard vim ex commands
  Vim.defineEx('w', '', () => forceSave())
  Vim.defineEx('q', '', () => { const currentNote = getCurrentNote(); if (currentNote) closeTab(currentNote) })
  Vim.defineEx('wq', '', () => { forceSave(); const currentNote = getCurrentNote(); if (currentNote) closeTab(currentNote) })
  Vim.defineEx('wa', '', () => forceSave())
  Vim.defineEx('tabn', '', () => {
    const currentNote = getCurrentNote()
    if (!currentNote) return
    const openTabs = getOpenTabs()
    const i = openTabs.indexOf(currentNote)
    const next = openTabs[i + 1] ?? openTabs[0]
    if (next) openTab(next)
  })
  Vim.defineEx('tabp', '', () => {
    const currentNote = getCurrentNote()
    if (!currentNote) return
    const openTabs = getOpenTabs()
    const i = openTabs.indexOf(currentNote)
    const prev = openTabs[i - 1] ?? openTabs[openTabs.length - 1]
    if (prev) openTab(prev)
  })
  Vim.defineEx('e', '', (_cm, params) => {
    const name = params.argString.trim()
    if (!name) return
    const notes = getNotes()
    const match = notes.find((n) => n === name || n.endsWith('/' + name) || n.endsWith('/' + name + '.md') || n === name + '.md')
    if (match) openTab(match)
  })

  Vim.map('<Space>w', ':w<CR>', 'normal')
  Vim.map('<Space>q', ':q<CR>', 'normal')
  Vim.map('<Space>e', ':knotesearch<CR>', 'normal')
  Vim.map('gt', ':tabn<CR>', 'normal')
  Vim.map('gT', ':tabp<CR>', 'normal')
}

export function relativeLineNumbers(vimMode: boolean): Extension {
  return lineNumbers({
    formatNumber(lineNo, state) {
      if (!vimMode) return String(lineNo)
      const mode = state.field(vimModeField, false) ?? 'normal'
      if (mode === 'insert') return String(lineNo)
      const curLine = state.doc.lineAt(state.selection.main.head).number
      const rel = Math.abs(lineNo - curLine)
      return rel === 0 ? String(lineNo) : String(rel)
    },
  })
}

export const livePreviewStyle = HighlightStyle.define([
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

export function formatCurrentTable(view: EditorView): boolean {
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

export function tableKeymap(): Extension {
  return Prec.highest(keymap.of([
    { key: 'Tab', run: tableNextCell },
    { key: 'Shift-Tab', run: tablePrevCell },
    { key: 'Enter', run: tableInsertRow },
  ]))
}

const wikilinkRe = /\[\[([^\]\[]+)\]\]/g

const wikilinkMark = Decoration.mark({ class: 'cm-wikilink' })

export function wikilinkPlugin(openFn: (name: string) => void): Extension {
  return ViewPlugin.fromClass(
    class {
      decorations
      constructor(view: EditorView) { this.decorations = this.build(view) }
      update(update: any) { if (update.docChanged || update.viewportChanged) this.decorations = this.build(update.view) }
      build(view: EditorView) {
        const b = new RangeSetBuilder<Decoration>()
        for (const { from, to } of view.visibleRanges) {
          const text = view.state.doc.sliceString(from, to)
          let m: RegExpExecArray | null
          wikilinkRe.lastIndex = 0
          while ((m = wikilinkRe.exec(text)) !== null) {
            b.add(from + m.index, from + m.index + m[0].length, wikilinkMark)
          }
        }
        return b.finish()
      }
    },
    {
      decorations: (v) => v.decorations,
      eventHandlers: {
        mousedown(e: MouseEvent, view: EditorView) {
          if (!(e.ctrlKey || e.metaKey)) return
          const pos = view.posAtCoords({ x: e.clientX, y: e.clientY })
          if (pos == null) return
          const line = view.state.doc.lineAt(pos)
          const text = line.text
          wikilinkRe.lastIndex = 0
          let m: RegExpExecArray | null
          while ((m = wikilinkRe.exec(text)) !== null) {
            const start = line.from + m.index
            const end = start + m[0].length
            if (pos >= start && pos <= end) {
              e.preventDefault()
              openFn(m[1])
              return
            }
          }
        },
      },
    }
  )
}

export function wikilinkCompletion(getNotes: () => string[]) {
  return autocompletion({
    override: [
      (ctx: CompletionContext) => {
        const before = ctx.matchBefore(/\[\[[^\]]*/)
        if (!before) return null
        const query = before.text.slice(2).toLowerCase()
        const options = getNotes()
          .filter((n) => n.toLowerCase().includes(query))
          .map((n) => ({ label: n, apply: `[[${n}]]`, type: 'keyword' }))
        return { from: before.from, options, validFor: /^\[\[[^\]]*/ }
      },
    ],
  })
}
