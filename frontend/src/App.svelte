<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, basicSetup } from 'codemirror'
  import { EditorState } from '@codemirror/state'
  import { markdown } from '@codemirror/lang-markdown'
  import { oneDark } from '@codemirror/theme-one-dark'
  import {
    RenderMarkdown,
    ListNotes,
    ReadNote,
    SaveNote,
    CreateNote,
    DeleteNote,
    RenameNote,
    SearchNotes,
    GetVaultPath,
    SelectVault,
    GetBacklinks,
  } from '../wailsjs/go/main/App.js'

  let notes: string[] = []
  let visibleNotes: string[] = []
  let currentNote: string | null = null
  let source = ''
  let html = ''
  let backlinks: string[] = []
  let newNoteName = ''
  let saveTimer: ReturnType<typeof setTimeout>
  let renamingNote: string | null = null
  let renameValue = ''
  let searchQuery = ''
  let searchTimer: ReturnType<typeof setTimeout>
  let vaultPath = ''

  function focusInput(el: HTMLInputElement): void {
    el.focus()
    el.select()
  }

  async function refreshList(): Promise<void> {
    notes = await ListNotes()
    await runSearch()
  }

  async function runSearch(): Promise<void> {
    visibleNotes = searchQuery.trim() ? await SearchNotes(searchQuery) : notes
  }

  function onSearchInput(): void {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(runSearch, 200)
  }

  async function selectNote(name: string): Promise<void> {
    source = await ReadNote(name)
    currentNote = name
    await render()
    backlinks = await GetBacklinks(name)
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

  function initEditor(el: HTMLDivElement): { destroy(): void } {
    const view = new EditorView({
      state: EditorState.create({
        doc: source,
        extensions: [
          basicSetup,
          markdown(),
          oneDark,
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
    return {
      destroy() {
        view.destroy()
      },
    }
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
      backlinks = []
    }
    await refreshList()
  }

  function startRename(name: string): void {
    renamingNote = name
    renameValue = name
  }

  function cancelRename(): void {
    renamingNote = null
  }

  async function confirmRename(): Promise<void> {
    const oldName = renamingNote
    const newName = renameValue.trim()
    renamingNote = null
    if (!oldName || !newName || newName === oldName) return
    await RenameNote(oldName, newName)
    if (currentNote === oldName) currentNote = newName
    await refreshList()
    if (currentNote) {
      source = await ReadNote(currentNote)
      await render()
      backlinks = await GetBacklinks(currentNote)
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
    await selectNote(name)
  }

  async function changeVault(): Promise<void> {
    vaultPath = await SelectVault()
    currentNote = null
    source = ''
    html = ''
    backlinks = []
    searchQuery = ''
    await refreshList()
  }

  onMount(async () => {
    vaultPath = await GetVaultPath()
    await refreshList()
  })
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
    <input
      class="search"
      bind:value={searchQuery}
      on:input={onSearchInput}
      placeholder="search"
    />
    <ul>
      {#each visibleNotes as name}
        <li class:active={name === currentNote}>
          {#if renamingNote === name}
            <input
              class="rename-input"
              use:focusInput
              bind:value={renameValue}
              on:keydown={(e) => {
                if (e.key === 'Enter') confirmRename()
                if (e.key === 'Escape') cancelRename()
              }}
              on:blur={confirmRename}
            />
          {:else}
            <span
              class="note-name"
              on:click={() => selectNote(name)}
              on:dblclick={() => startRename(name)}
            >{name}</span>
          {/if}
          <button class="delete" on:click={() => deleteNote(name)}>×</button>
        </li>
      {/each}
    </ul>
    <div class="vault" title={vaultPath}>
      <span class="vault-path">{vaultPath}</span>
      <button on:click={changeVault}>変更</button>
    </div>
  </nav>

  {#if currentNote}
    {#key currentNote}
      <div class="editor" use:initEditor></div>
    {/key}
    <div class="preview">
      <div on:click={onPreviewClick}>{@html html}</div>
      {#if backlinks.length}
        <div class="backlinks">
          <div class="backlinks-title">バックリンク</div>
          <ul>
            {#each backlinks as name}
              <li><span class="note-name" on:click={() => selectNote(name)}>{name}</span></li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
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
    display: flex;
    flex-direction: column;
    min-height: 0;
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

  .search {
    margin: 0 0.5rem 0.5rem;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    flex: 1;
    overflow-y: auto;
  }

  .vault {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.4rem 0.6rem;
    border-top: 1px solid #ccc;
    font-size: 0.75rem;
  }

  .vault-path {
    flex: 1;
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
    background: rgba(128, 128, 128, 0.2);
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
    overflow: hidden;
    min-width: 0;
  }

  .editor :global(.cm-editor) {
    height: 100%;
  }

  .editor :global(.cm-scroller) {
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

  .backlinks {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid #444;
  }

  .backlinks-title {
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
    color: #7c9eff;
  }

  .empty {
    grid-column: 2 / 4;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.5;
  }
</style>
