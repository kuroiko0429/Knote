<script lang="ts" context="module">
  export type TreeNode =
    | { type: 'folder'; name: string; path: string; children: TreeNode[] }
    | { type: 'note'; name: string; path: string }
</script>

<script lang="ts">
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
  export let onDeleteNote: (path: string) => void
  export let focusInputAction: (el: HTMLInputElement) => void

  $: indent = `${depth * 0.9}rem`
</script>

{#if node.type === 'folder'}
  <li
    class="tree-row folder"
    style="padding-left:{indent}"
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
        {expanded.has(node.path) ? '▾' : '▸'} {node.name}
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
        {onDeleteNote}
        {focusInputAction}
      />
    {/each}
  {/if}
{:else}
  <li
    class="tree-row"
    class:active={node.path === currentNote}
    style="padding-left:{indent}"
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
      >{node.name}</span>
    {/if}
    <button class="delete" on:click={() => onDeleteNote(node.path)}>×</button>
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
    background: rgba(128, 128, 128, 0.2);
  }

  .folder-name,
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
</style>
