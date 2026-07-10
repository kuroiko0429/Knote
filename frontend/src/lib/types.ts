export interface BacklinkItem { note: string; snippets: string[] }

export interface SnippetDef { trigger: string; name: string; content: string }

export type PaletteItem =
  | { kind: 'cmd'; label: string; shortcut?: string; action: () => void }
  | { kind: 'note'; path: string }
  | { kind: 'create'; path: string }

export interface OutlineItem {
  level: number
  text: string
  el: HTMLElement
}

export type SidebarRefreshDetail =
  | { kind: 'pathChange'; type: 'note' | 'folder'; oldPath: string; newPath: string }
  | { kind: 'moveModal'; oldPath: string; newPath: string }
  | { kind: 'delete'; path: string }
  | undefined
