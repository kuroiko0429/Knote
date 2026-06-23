<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import { StartTerminal, WriteTerminal, ResizeTerminal } from '../wailsjs/go/main/App.js'
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'

  export let visible: boolean

  let container: HTMLDivElement
  let term: Terminal | null = null
  let fitAddon: FitAddon | null = null
  let resizeObserver: ResizeObserver

  onMount(async () => {
    term = new Terminal({
      fontSize: 13,
      fontFamily: 'monospace',
      cursorBlink: true,
      theme: { background: '#1b2636' },
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
