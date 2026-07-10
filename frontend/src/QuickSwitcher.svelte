<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import { FileText, FilePlus, Code2 } from 'lucide-svelte'
  import type { PaletteItem } from './lib/types'

  export let notes: string[]
  export let commands: PaletteItem[]

  const dispatch = createEventDispatcher<{ select: PaletteItem; close: void }>()

  let qsQuery = ''
  let qsIndex = 0
  let qsInputEl: HTMLInputElement
  $: paletteItems = (() => {
    const q = qsQuery.trim()
    const ql = q.toLowerCase()
    const items: PaletteItem[] = []
    if (q.startsWith('>')) {
      const cmdQ = ql.slice(1).trim()
      for (const cmd of commands) {
        if (cmd.kind === 'cmd' && (!cmdQ || cmd.label.toLowerCase().includes(cmdQ))) {
          items.push({ kind: 'cmd', label: cmd.label, shortcut: cmd.shortcut, action: cmd.action })
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

  onMount(() => {
    qsInputEl?.focus()
  })

  function selectItem(item: PaletteItem): void {
    dispatch('select', item)
  }

  async function onQsKeydown(e: KeyboardEvent): Promise<void> {
    if (e.key === 'Escape') {
      e.preventDefault()
      dispatch('close')
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      qsIndex = Math.min(qsIndex + 1, qsTotal - 1)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      qsIndex = Math.max(qsIndex - 1, 0)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const item = paletteItems[qsIndex]
      if (item) selectItem(item)
    }
  }
</script>

<div class="modal-overlay qs-overlay" on:click={() => dispatch('close')}>
  <div class="quick-switcher" on:click|stopPropagation>
    <input
      bind:this={qsInputEl}
      bind:value={qsQuery}
      on:keydown={onQsKeydown}
      class="qs-input"
      placeholder={qsQuery.startsWith('>') ? 'コマンドを検索...' : 'ノートを開く... (「>」でコマンド)'}
    />
    <ul class="qs-list">
      {#if qsQuery.startsWith('>')}
        <li class="qs-section">コマンド</li>
      {:else if paletteItems.some(i => i.kind === 'note' || i.kind === 'create')}
        <li class="qs-section">ノート</li>
      {/if}
      {#each paletteItems as item, i}
        <li class:active={i === qsIndex} on:click={() => selectItem(item)}>
          {#if item.kind === 'cmd'}
            <Code2 size={13} />
            <span class="qs-label">{item.label}</span>
            {#if item.shortcut}<span class="qs-shortcut">{item.shortcut}</span>{/if}
          {:else if item.kind === 'note'}
            <FileText size={13} /><span class="qs-label">{item.path}</span>
          {:else if item.kind === 'create'}
            <FilePlus size={13} /><span class="qs-label">新規作成: "{item.path}"</span>
          {/if}
        </li>
      {/each}
      {#if paletteItems.length === 0}
        <li class="qs-empty">一致する項目がありません</li>
      {/if}
    </ul>
  </div>
</div>

<style>
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

  .qs-shortcut {
    font-size: 0.72rem;
    color: var(--text-dim);
    background: var(--bg-hover);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0.05rem 0.3rem;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .qs-section {
    font-size: 0.7rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 0.4rem 0.6rem 0.2rem;
    cursor: default;
    border-top: 1px solid var(--border);
  }

  .qs-section:first-child {
    border-top: none;
  }
</style>
