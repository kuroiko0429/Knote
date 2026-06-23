<script lang="ts" context="module">
  export type TreeNode =
    | { type: 'folder'; name: string; path: string; children: TreeNode[] }
    | { type: 'note'; name: string; path: string }
</script>

<script lang="ts">
  import { ChevronRight, ChevronDown, Folder, FolderOpen, FileText } from 'lucide-svelte'

  export let node: TreeNode
  export let depth: number
  export let currentNote: string | null
  export let renamingPath: string | null
  export let renamingType: 'note' | 'folder' | null
  export let renameValue: string
  export let expanded: Set<string>
  export let onSelect: (path: string) => void
  export let onToggle: (path: string) => void
  export let onRenameStartNote: (path: string) => void
  export let onRenameStartFolder: (path: string) => void
  export let onConfirmRename: () => void
  export let onCancelRename: () => void
  export let onNoteContext: (e: MouseEvent, path: string) => void
  export let onFolderContext: (e: MouseEvent, path: string) => void
  export let onDrop: (targetFolder: string, e: DragEvent) => void
  export let focusInputAction: (el: HTMLInputElement) => void

  $: indent = `${depth * 0.9}rem`

  let dragDepth = 0
  $: dragOver = dragDepth > 0

  function onDragStart(e: DragEvent): void {
    e.dataTransfer?.setData('text/plain', JSON.stringify({ path: node.path, type: node.type }))
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }

  function onDragEnter(): void {
    dragDepth++
  }

  function onDragLeave(): void {
    dragDepth = Math.max(0, dragDepth - 1)
  }

  function parentOf(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? '' : path.slice(0, i)
  }
</script>

{#if node.type === 'folder'}
  <li
    class="tree-row folder"
    class:drag-over={dragOver}
    style="padding-left:{indent}"
    draggable="true"
    on:dragstart={onDragStart}
    on:dragover={(e) => e.preventDefault()}
    on:dragenter={onDragEnter}
    on:dragleave={onDragLeave}
    on:drop={(e) => {
      dragDepth = 0
      onDrop(node.path, e)
    }}
    on:contextmenu={(e) => onFolderContext(e, node.path)}
  >
    {#if renamingPath === node.path && renamingType === 'folder'}
      <input
        class="rename-input"
        use:focusInputAction
        bind:value={renameValue}
        on:keydown={(e) => {
          if (e.key === 'Enter') onConfirmRename()
          if (e.key === 'Escape') onCancelRename()
        }}
        on:blur={onConfirmRename}
      />
    {:else}
      <span class="folder-name" on:click={() => onToggle(node.path)}>
        <svelte:component this={expanded.has(node.path) ? ChevronDown : ChevronRight} size={14} class="chevron" />
        <svelte:component this={expanded.has(node.path) ? FolderOpen : Folder} size={15} />
        {node.name}
      </span>
    {/if}
  </li>
  {#if expanded.has(node.path)}
    {#each node.children as child (child.path)}
      <svelte:self
        node={child}
        depth={depth + 1}
        {currentNote}
        {renamingPath}
        {renamingType}
        bind:renameValue
        {expanded}
        {onSelect}
        {onToggle}
        {onRenameStartNote}
        {onRenameStartFolder}
        {onConfirmRename}
        {onCancelRename}
        {onNoteContext}
        {onFolderContext}
        {onDrop}
        {focusInputAction}
      />
    {/each}
  {/if}
{:else}
  <li
    class="tree-row"
    class:active={node.path === currentNote}
    class:drag-over={dragOver}
    style="padding-left:{indent}"
    draggable="true"
    on:dragstart={onDragStart}
    on:dragover={(e) => e.preventDefault()}
    on:dragenter={onDragEnter}
    on:dragleave={onDragLeave}
    on:drop={(e) => {
      dragDepth = 0
      onDrop(parentOf(node.path), e)
    }}
    on:contextmenu={(e) => onNoteContext(e, node.path)}
  >
    {#if renamingPath === node.path && renamingType === 'note'}
      <input
        class="rename-input"
        use:focusInputAction
        bind:value={renameValue}
        on:keydown={(e) => {
          if (e.key === 'Enter') onConfirmRename()
          if (e.key === 'Escape') onCancelRename()
        }}
        on:blur={onConfirmRename}
      />
    {:else}
      <span
        class="note-name"
        on:click={() => onSelect(node.path)}
        on:dblclick={() => onRenameStartNote(node.path)}
      >
        <FileText size={14} />
        {node.name}
      </span>
    {/if}
  </li>
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: 0.4rem;
    padding-bottom: 0.4rem;
    padding-right: 0.6rem;
    cursor: pointer;
  }

  .tree-row.active {
    background: var(--bg-hover);
  }

  .tree-row.drag-over {
    background: var(--accent-hover);
    outline: 1px dashed var(--accent);
    outline-offset: -1px;
  }

  .folder-name,
  .note-name {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .folder-name :global(.chevron) {
    flex-shrink: 0;
    opacity: 0.6;
  }

  .folder-name :global(svg),
  .note-name :global(svg) {
    flex-shrink: 0;
    opacity: 0.7;
  }

  .rename-input {
    flex: 1;
    min-width: 0;
  }

</style>
