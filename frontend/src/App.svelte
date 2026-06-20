<script lang="ts">
  import { onMount } from 'svelte'
  import {
    RenderMarkdown,
    ListNotes,
    ReadNote,
    SaveNote,
    CreateNote,
    DeleteNote,
  } from '../wailsjs/go/main/App.js'

  let notes: string[] = []
  let currentNote: string | null = null
  let source = ''
  let html = ''
  let newNoteName = ''
  let saveTimer: ReturnType<typeof setTimeout>

  async function refreshList(): Promise<void> {
    notes = await ListNotes()
  }

  async function selectNote(name: string): Promise<void> {
    currentNote = name
    source = await ReadNote(name)
    await render()
  }

  async function render(): Promise<void> {
    html = await RenderMarkdown(source)
  }

  function onEdit(): void {
    render()
    if (!currentNote) return
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => SaveNote(currentNote!, source), 400)
  }

  async function newNote(): Promise<void> {
    const name = newNoteName.trim()
    if (!name) return
    await CreateNote(name)
    newNoteName = ''
    await refreshList()
    await selectNote(name)
  }

  async function deleteNote(name: string): Promise<void> {
    await DeleteNote(name)
    if (currentNote === name) {
      currentNote = null
      source = ''
      html = ''
    }
    await refreshList()
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
    await selectNote(name)
  }

  onMount(refreshList)
</script>

<main>
  <nav class="sidebar">
    <div class="new-note">
      <input
        bind:value={newNoteName}
        on:keydown={(e) => e.key === 'Enter' && newNote()}
        placeholder="new note name"
      />
      <button on:click={newNote}>+</button>
    </div>
    <ul>
      {#each notes as name}
        <li class:active={name === currentNote}>
          <span class="note-name" on:click={() => selectNote(name)}>{name}</span>
          <button class="delete" on:click={() => deleteNote(name)}>×</button>
        </li>
      {/each}
    </ul>
  </nav>

  {#if currentNote}
    <textarea bind:value={source} on:input={onEdit} class="editor"></textarea>
    <div class="preview" on:click={onPreviewClick}>{@html html}</div>
  {:else}
    <div class="empty">ノートを選ぶか、新規作成して</div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
  }

  main {
    display: grid;
    grid-template-columns: 200px 1fr 1fr;
    height: 100vh;
  }

  .sidebar {
    border-right: 1px solid #ccc;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .new-note {
    display: flex;
    padding: 0.5rem;
    gap: 0.25rem;
  }

  .new-note input {
    flex: 1;
    min-width: 0;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 0.6rem;
    cursor: pointer;
  }

  li.active {
    background: rgba(128, 128, 128, 0.2);
  }

  .note-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

  .editor {
    border: none;
    outline: none;
    resize: none;
    padding: 1rem;
    font-family: monospace;
    font-size: 1rem;
  }

  .preview {
    padding: 1rem;
    overflow-y: auto;
    border-left: 1px solid #ccc;
  }

  .preview :global(a[href^='knote:']) {
    color: #7c9eff;
    text-decoration: none;
    cursor: pointer;
  }

  .preview :global(a[href^='knote:']:hover) {
    text-decoration: underline;
  }

  .empty {
    grid-column: 2 / 4;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }
</style>
