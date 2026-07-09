export interface BacklinkItem { note: string; snippets: string[] }

export interface SnippetDef { trigger: string; name: string; content: string }

export type PaletteItem =
  | { kind: 'cmd'; label: string; shortcut?: string; action: () => void }
  | { kind: 'note'; path: string }
  | { kind: 'create'; path: string }
  | { kind: 'rename'; value: string }
  | { kind: 'newFolder'; value: string }

export interface OutlineItem {
  level: number
  text: string
  el: HTMLElement
}
