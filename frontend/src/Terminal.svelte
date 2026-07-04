<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import { StartTerminal, WriteTerminal, ResizeTerminal } from '../wailsjs/go/main/App.js'
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'

  export let visible: boolean

  let container: HTMLDivElement

  export function refreshTheme() {
    if (!term) return
    const s = getComputedStyle(document.documentElement)
    const newTheme = {
      background:          s.getPropertyValue('--bg').trim() || '#1b2636',
      foreground:          s.getPropertyValue('--text').trim() || '#ffffff',
      cursor:              s.getPropertyValue('--text').trim() || '#ffffff',
      selectionBackground: s.getPropertyValue('--bg-hover').trim() || 'rgba(255,255,255,0.1)',
    }
    // xterm v5/v6 両対応
    try { term.options.theme = newTheme } catch {}
    try { (term as any).setOption?.('theme', newTheme) } catch {}
  }
  let term: Terminal | null = null
  let fitAddon: FitAddon | null = null
  let resizeObserver: ResizeObserver

  onMount(async () => {
    const s = getComputedStyle(document.documentElement)
    term = new Terminal({
      fontSize: 13,
      fontFamily: 'monospace',
      cursorBlink: true,
      theme: {
        background: s.getPropertyValue('--bg').trim(),
        foreground: s.getPropertyValue('--text').trim(),
        cursor:     s.getPropertyValue('--text').trim(),
      },
    })
    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(container)

    term.onData((data) => {
      WriteTerminal(data)
    })

    EventsOn('terminal:data', (data: string) => {
      term?.write(data)
    })

    resizeObserver = new ResizeObserver(() => {
      if (!visible || !fitAddon || !term) return
      fitAddon.fit()
      ResizeTerminal(term.cols, term.rows)
    })
    resizeObserver.observe(container)

    if (visible) {
      await tick()
      fitAddon.fit()
    }
    await StartTerminal()
    if (term) ResizeTerminal(term.cols, term.rows)
  })

  $: if (visible && fitAddon && term) {
    tick().then(() => {
      fitAddon?.fit()
      if (term) ResizeTerminal(term.cols, term.rows)
    })
  }

  onDestroy(() => {
    resizeObserver?.disconnect()
    EventsOff('terminal:data')
    term?.dispose()
  })
</script>

<div class="terminal-container" bind:this={container}></div>

<style>
  .terminal-container {
    flex: 1;
    min-height: 0;
    width: 100%;
    padding: 0.3rem;
    box-sizing: border-box;
  }
</style>
