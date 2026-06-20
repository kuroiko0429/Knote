<script lang="ts">
  import { RenderMarkdown } from '../wailsjs/go/main/App.js'

  let source: string = '# Knote\n\nここにMarkdownを書く'
  let html: string = ''

  async function update(): Promise<void> {
    html = await RenderMarkdown(source)
  }

  update()
</script>

<main>
  <textarea bind:value={source} on:input={update} class="editor"></textarea>
  <div class="preview">{@html html}</div>
</main>

<style>
  :global(body) {
    margin: 0;
  }

  main {
    display: grid;
    grid-template-columns: 1fr 1fr;
    height: 100vh;
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
</style>
