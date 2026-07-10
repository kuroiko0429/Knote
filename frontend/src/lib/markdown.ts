import renderMathInElement from 'katex/contrib/auto-render'
import mermaid from 'mermaid'
import { QueryNotes } from '../../wailsjs/go/main/App.js'

export function applyCheckboxes(root: HTMLElement, source: string, onSourceChange: (next: string) => void): void {
  if (!root) return
  const boxes = Array.from(root.querySelectorAll('input[type="checkbox"]')) as HTMLInputElement[]
  boxes.forEach((cb, idx) => {
    cb.removeAttribute('disabled')
    cb.addEventListener('click', (e) => {
      e.preventDefault()
      let count = 0
      const newSource = source.replace(/^(\s*[-*+]\s+)\[([xX ])\]/gm, (match, prefix, state) => {
        if (count++ === idx) return `${prefix}[${state.trim() === '' ? 'x' : ' '}]`
        return match
      })
      if (newSource === source) return
      onSourceChange(newSource)
    })
  })
}

export function applyMath(root: HTMLElement): void {
  if (!root) return
  renderMathInElement(root, {
    delimiters: [
      { left: '$$', right: '$$', display: true },
      { left: '$', right: '$', display: false },
    ],
    throwOnError: false,
  })
}

let mermaidInitialized = false

export async function applyMermaid(root: HTMLElement, theme: 'dark' | 'light'): Promise<void> {
  if (!root) return
  if (!mermaidInitialized) {
    mermaid.initialize({ startOnLoad: false, theme: theme === 'dark' ? 'dark' : 'default' })
    mermaidInitialized = true
  }
  const blocks = Array.from(
    root.querySelectorAll('code.language-mermaid')
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

function dvEscape(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export async function applyDataview(root: HTMLElement, onOpenNote: (name: string) => void): Promise<void> {
  if (!root) return
  const blocks = Array.from(
    root.querySelectorAll('pre > code.language-dataview')
  ) as HTMLElement[]
  for (const code of blocks) {
    const pre = code.parentElement!
    const query = code.textContent ?? ''
    const result = await QueryNotes(query)

    const container = document.createElement('div')
    container.className = 'dataview-result'

    if (result.error) {
      container.innerHTML = `<div class="dataview-error">${dvEscape(result.error)}</div>`
    } else if (result.mode === 'table') {
      const cols = result.columns?.length ? result.columns : ['file.name']
      let out = '<table class="dataview-table"><thead><tr>'
      for (const col of cols) {
        out += `<th>${dvEscape(col)}</th>`
      }
      out += '</tr></thead><tbody>'
      for (const row of result.rows ?? []) {
        out += '<tr>'
        for (const col of cols) {
          const val = row[col] ?? ''
          if (col === 'file' || col === 'file.name') {
            const fp = row['file'] ?? ''
            const name = row['file.name'] ?? fp
            out += `<td><a class="dv-link" data-path="${dvEscape(fp)}">${dvEscape(name)}</a></td>`
          } else {
            out += `<td>${dvEscape(val)}</td>`
          }
        }
        out += '</tr>'
      }
      out += '</tbody></table>'
      if ((result.rows?.length ?? 0) === 0) {
        out += '<div class="dataview-empty">該当なし</div>'
      }
      container.innerHTML = out
    } else {
      // LIST
      let out = '<ul class="dataview-list">'
      for (const row of result.rows ?? []) {
        const fp = row['file'] ?? ''
        const name = row['file.name'] ?? fp
        out += `<li><a class="dv-link" data-path="${dvEscape(fp)}">${dvEscape(name)}</a></li>`
      }
      out += '</ul>'
      if ((result.rows?.length ?? 0) === 0) {
        out += '<div class="dataview-empty">該当なし</div>'
      }
      container.innerHTML = out
    }

    // wire up note links
    for (const a of Array.from(container.querySelectorAll('a.dv-link')) as HTMLAnchorElement[]) {
      const p = a.dataset.path ?? ''
      a.addEventListener('click', (e) => {
        e.preventDefault()
        if (p) onOpenNote(p.replace(/\.md$/, ''))
      })
    }

    pre.replaceWith(container)
  }
}

export function applyCodeBlockButtons(root: HTMLElement, onRun: (lang: string, code: string) => void): void {
  if (!root) return
  for (const pre of Array.from(root.querySelectorAll('pre')) as HTMLElement[]) {
    const code = pre.querySelector('code')
    if (!code) continue
    pre.style.position = 'relative'
    const btn = document.createElement('button')
    btn.className = 'code-run-btn'
    btn.textContent = '▶ run'
    btn.title = 'ターミナルで実行'
    btn.addEventListener('click', (e) => {
      e.stopPropagation()
      const text = (code.textContent || '').trimEnd()
      if (!text) return
      const lang = (code.className || '').replace('language-', '').split(' ')[0]
      onRun(lang, text)
    })
    pre.appendChild(btn)
  }
}
