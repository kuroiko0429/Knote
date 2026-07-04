<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { CheckSquare, Square, Plus, X, GripVertical } from 'lucide-svelte'

  export let source: string

  const dispatch = createEventDispatcher<{ change: string }>()

  let skipNextSourceUpdate = false

  interface KanbanCard {
    text: string
    done: boolean
  }

  interface KanbanColumn {
    title: string
    cards: KanbanCard[]
  }

  let preamble = ''
  let columns: KanbanColumn[] = []

  $: if (source) {
    if (skipNextSourceUpdate) {
      skipNextSourceUpdate = false
    } else {
      const parsed = parseSource(source)
      preamble = parsed.preamble
      columns = parsed.columns
    }
  }

  function parseSource(src: string): { preamble: string; columns: KanbanColumn[] } {
    const lines = src.split('\n')
    const preambleLines: string[] = []
    const cols: KanbanColumn[] = []
    let currentCol: KanbanColumn | null = null
    let inFrontmatter = false
    let frontmatterDone = false

    for (const line of lines) {
      if (!frontmatterDone && line.trim() === '---') {
        if (!inFrontmatter) {
          inFrontmatter = true
          preambleLines.push(line)
          continue
        } else {
          frontmatterDone = true
          preambleLines.push(line)
          continue
        }
      }
      if (inFrontmatter && !frontmatterDone) {
        preambleLines.push(line)
        continue
      }
      if (line.startsWith('## ')) {
        currentCol = { title: line.slice(3).trim(), cards: [] }
        cols.push(currentCol)
      } else if (currentCol && /^- \[[ xX]\]/.test(line)) {
        const done = line[3] !== ' '
        const text = line.slice(6).trim()
        currentCol.cards.push({ text, done })
      } else if (!currentCol) {
        preambleLines.push(line)
      }
    }

    return { preamble: preambleLines.join('\n'), columns: cols }
  }

  function serialize(): string {
    const parts: string[] = [preamble]
    for (const col of columns) {
      parts.push(`\n## ${col.title}`)
      for (const card of col.cards) {
        parts.push(`- [${card.done ? 'x' : ' '}] ${card.text}`)
      }
    }
    return parts.join('\n')
  }

  function emit() {
    skipNextSourceUpdate = true
    dispatch('change', serialize())
  }

  function toggleCard(ci: number, ki: number) {
    columns[ci].cards[ki].done = !columns[ci].cards[ki].done
    columns = columns
    emit()
  }

  function removeCard(ci: number, ki: number) {
    columns[ci].cards.splice(ki, 1)
    columns = columns
    emit()
  }

  function removeColumn(ci: number) {
    columns.splice(ci, 1)
    columns = columns
    emit()
  }

  let newCardText: string[] = []
  let addingCard: number | null = null

  function startAddCard(ci: number) {
    addingCard = ci
    newCardText[ci] = ''
  }

  function commitAddCard(ci: number) {
    const text = (newCardText[ci] ?? '').trim()
    if (text) {
      columns[ci].cards.push({ text, done: false })
      columns = columns
      emit()
    }
    addingCard = null
  }

  let addingColumn = false
  let newColumnTitle = ''

  function commitAddColumn() {
    const title = newColumnTitle.trim()
    if (title) {
      columns = [...columns, { title, cards: [] }]
      emit()
    }
    addingColumn = false
    newColumnTitle = ''
  }

  // drag & drop
  let dragCol: number | null = null
  let dragCard: number | null = null
  let dropCol: number | null = null
  let dropCard: number | null = null

  function onDragStart(ci: number, ki: number, e: DragEvent) {
    dragCol = ci
    dragCard = ki
    e.dataTransfer!.effectAllowed = 'move'
  }

  function onDragOver(ci: number, ki: number, e: DragEvent) {
    e.preventDefault()
    dropCol = ci
    dropCard = ki
  }

  function onDropOnCard(ci: number, ki: number) {
    if (dragCol === null || dragCard === null) return
    const card = columns[dragCol].cards.splice(dragCard, 1)[0]
    const targetIdx = dragCol === ci && dragCard < ki ? ki - 1 : ki
    columns[ci].cards.splice(targetIdx, 0, card)
    columns = columns
    dragCol = dragCard = dropCol = dropCard = null
    emit()
  }

  function onDropOnColumn(ci: number, e: DragEvent) {
    e.preventDefault()
    if (dragCol === null || dragCard === null) return
    if (dropCard !== null) return
    const card = columns[dragCol].cards.splice(dragCard, 1)[0]
    columns[ci].cards.push(card)
    columns = columns
    dragCol = dragCard = dropCol = dropCard = null
    emit()
  }

  function onDragEnd() {
    dragCol = dragCard = dropCol = dropCard = null
  }

  function onCardKeydown(e: KeyboardEvent, ci: number) {
    if (e.key === 'Enter') { e.preventDefault(); commitAddCard(ci) }
    if (e.key === 'Escape') { addingCard = null }
  }

  function onColKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); commitAddColumn() }
    if (e.key === 'Escape') { addingColumn = false }
  }
</script>

<div class="kanban">
  {#each columns as col, ci}
    <div
      class="column"
      on:dragover={(e) => { e.preventDefault(); if (dropCard === null) dropCol = ci }}
      on:drop={(e) => onDropOnColumn(ci, e)}
    >
      <div class="col-header">
        <span class="col-title">{col.title}</span>
        <span class="col-count">{col.cards.length}</span>
        <button class="icon-btn" on:click={() => removeColumn(ci)}><X size={13} /></button>
      </div>

      <div class="cards">
        {#each col.cards as card, ki}
          <div
            class="card"
            class:done={card.done}
            class:drag-over={dropCol === ci && dropCard === ki}
            draggable="true"
            on:dragstart={(e) => onDragStart(ci, ki, e)}
            on:dragover={(e) => onDragOver(ci, ki, e)}
            on:drop={() => onDropOnCard(ci, ki)}
            on:dragend={onDragEnd}
          >
            <span class="drag-handle"><GripVertical size={13} /></span>
            <button class="checkbox" on:click={() => toggleCard(ci, ki)}>
              {#if card.done}
                <CheckSquare size={15} />
              {:else}
                <Square size={15} />
              {/if}
            </button>
            <span class="card-text" class:done-text={card.done}>{card.text}</span>
            <button class="icon-btn remove-card" on:click={() => removeCard(ci, ki)}><X size={12} /></button>
          </div>
        {/each}

        {#if addingCard === ci}
          <div class="card new-card-input">
            <input
              type="text"
              bind:value={newCardText[ci]}
              placeholder="タスクを入力..."
              on:keydown={(e) => onCardKeydown(e, ci)}
              autofocus
            />
            <button class="icon-btn" on:click={() => commitAddCard(ci)}><Plus size={13} /></button>
            <button class="icon-btn" on:click={() => (addingCard = null)}><X size={13} /></button>
          </div>
        {:else}
          <button class="add-card-btn" on:click={() => startAddCard(ci)}>
            <Plus size={13} /> カードを追加
          </button>
        {/if}
      </div>
    </div>
  {/each}

  {#if addingColumn}
    <div class="column new-col">
      <input
        type="text"
        bind:value={newColumnTitle}
        placeholder="カラム名..."
        on:keydown={onColKeydown}
        autofocus
      />
      <div class="new-col-actions">
        <button on:click={commitAddColumn}>追加</button>
        <button on:click={() => (addingColumn = false)}><X size={13} /></button>
      </div>
    </div>
  {:else}
    <button class="add-col-btn" on:click={() => (addingColumn = true)}>
      <Plus size={14} /> カラムを追加
    </button>
  {/if}
</div>

<style>
  .kanban {
    display: flex;
    gap: 1rem;
    padding: 1rem;
    flex: 1;
    overflow-x: auto;
    overflow-y: hidden;
    align-items: flex-start;
  }

  .column {
    flex-shrink: 0;
    width: 260px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 10rem);
  }

  .col-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.6rem 0.75rem;
    border-bottom: 1px solid var(--border);
    font-weight: 600;
    font-size: 0.85rem;
  }

  .col-title {
    flex: 1;
  }

  .col-count {
    font-size: 0.75rem;
    color: var(--text-dim);
    background: var(--bg);
    border-radius: 10px;
    padding: 0.1rem 0.4rem;
  }

  .cards {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    padding: 0.5rem;
    overflow-y: auto;
    flex: 1;
  }

  .card {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.45rem 0.5rem;
    cursor: grab;
    user-select: none;
  }

  .card:active {
    cursor: grabbing;
  }

  .card.drag-over {
    border-color: var(--accent);
    background: var(--bg-hover);
  }

  .card.done {
    opacity: 0.6;
  }

  .drag-handle {
    color: var(--text-dim);
    flex-shrink: 0;
    display: flex;
    align-items: center;
  }

  .checkbox {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--accent);
    display: flex;
    align-items: center;
    flex-shrink: 0;
  }

  .card-text {
    flex: 1;
    font-size: 0.85rem;
    line-height: 1.4;
    word-break: break-word;
  }

  .done-text {
    text-decoration: line-through;
    color: var(--text-dim);
  }

  .remove-card {
    opacity: 0;
    flex-shrink: 0;
  }

  .card:hover .remove-card {
    opacity: 1;
  }

  .icon-btn {
    background: none;
    border: none;
    padding: 2px;
    cursor: pointer;
    color: var(--text-dim);
    display: flex;
    align-items: center;
    border-radius: 3px;
  }

  .icon-btn:hover {
    color: var(--text);
    background: var(--bg-hover);
  }

  .add-card-btn {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    width: 100%;
    background: none;
    border: 1px dashed var(--border);
    border-radius: 6px;
    padding: 0.4rem 0.5rem;
    font-size: 0.8rem;
    color: var(--text-dim);
    cursor: pointer;
  }

  .add-card-btn:hover {
    background: var(--bg-hover);
    color: var(--text);
  }

  .new-card-input {
    cursor: default;
  }

  .new-card-input input {
    flex: 1;
    background: none;
    border: none;
    color: var(--text);
    font-size: 0.85rem;
    outline: none;
    min-width: 0;
  }

  .add-col-btn {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--bg-secondary);
    border: 1px dashed var(--border);
    border-radius: 8px;
    padding: 0.6rem 1rem;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.85rem;
    white-space: nowrap;
    align-self: flex-start;
  }

  .add-col-btn:hover {
    background: var(--bg-hover);
    color: var(--text);
  }

  .new-col {
    padding: 0.75rem;
    gap: 0.5rem;
    width: 220px;
  }

  .new-col input {
    background: var(--bg);
    border: 1px solid var(--accent);
    border-radius: 4px;
    color: var(--text);
    font-size: 0.9rem;
    padding: 0.3rem 0.5rem;
    outline: none;
    width: 100%;
  }

  .new-col-actions {
    display: flex;
    gap: 0.4rem;
  }

  .new-col-actions button {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text);
    padding: 0.2rem 0.5rem;
    cursor: pointer;
    font-size: 0.8rem;
    display: flex;
    align-items: center;
  }

  .new-col-actions button:hover {
    background: var(--bg-hover);
  }
</style>
