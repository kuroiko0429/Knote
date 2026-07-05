<script lang="ts">
  import { onDestroy, createEventDispatcher } from 'svelte'
  import {
    forceSimulation,
    forceLink,
    forceManyBody,
    forceCenter,
    forceCollide,
    forceX,
    forceY,
    type Simulation,
    type SimulationNodeDatum,
  } from 'd3-force'

  export let notes: string[]
  export let edges: { source: string; target: string }[]
  export let currentNote: string | null

  interface Node extends SimulationNodeDatum {
    id: string
    exists: boolean
    degree: number
    folder: string
  }
  interface Link {
    source: Node
    target: Node
  }

  const dispatch = createEventDispatcher<{ select: string }>()

  let container: HTMLDivElement
  let width = 600
  let height = 600
  let nodes: Node[] = []
  let links: Link[] = []
  let simulation: Simulation<Node, undefined> | null = null
  let dragNode: Node | null = null
  let hoveredNode: Node | null = null
  let maxDepth = 0

  let vb = { x: 0, y: 0, w: 600, h: 600 }
  let vbInitialized = false
  let panning = false
  let panStart = { x: 0, y: 0, vbX: 0, vbY: 0 }

  const PALETTE = [
    '#7c9eff', '#ff79c6', '#ffb86c', '#50fa7b',
    '#bd93f9', '#8be9fd', '#ff5555', '#f1fa8c',
    '#ff92df', '#69ff47',
  ]

  function hashFolder(s: string): number {
    let h = 0
    for (let i = 0; i < s.length; i++) h = (Math.imul(31, h) + s.charCodeAt(i)) | 0
    return Math.abs(h)
  }

  function colorFor(folder: string): string {
    if (folder === '') return PALETTE[0]
    return PALETTE[1 + (hashFolder(folder) % (PALETTE.length - 1))]
  }

  function folderOf(id: string): string {
    const i = id.lastIndexOf('/')
    return i === -1 ? '' : id.slice(0, i)
  }

  function labelOf(id: string): string {
    const i = id.lastIndexOf('/')
    return i === -1 ? id : id.slice(i + 1)
  }

  function nodeRadius(n: Node): number {
    const base = n.id === currentNote ? 10 : 6
    return Math.min(base + n.degree * 1.2, 18)
  }

  // ホバー時の隣接ノード
  $: connectedToHovered = (() => {
    if (!hoveredNode) return new Set<string>()
    const s = new Set<string>()
    for (const l of links) {
      if (l.source.id === hoveredNode.id) s.add(l.target.id)
      if (l.target.id === hoveredNode.id) s.add(l.source.id)
    }
    return s
  })()

  // 深さフィルタ: currentNote から BFS
  $: visibleIds = (() => {
    if (maxDepth === 0 || !currentNote) return null
    const dist = new Map<string, number>([[currentNote, 0]])
    const queue = [currentNote]
    while (queue.length) {
      const cur = queue.shift()!
      const d = dist.get(cur)!
      if (d >= maxDepth) continue
      for (const l of links) {
        const nb = l.source.id === cur ? l.target.id : l.target.id === cur ? l.source.id : null
        if (nb && !dist.has(nb)) { dist.set(nb, d + 1); queue.push(nb) }
      }
    }
    return dist
  })()

  function visible(id: string): boolean {
    return !visibleIds || visibleIds.has(id)
  }

  function nodeOpacity(n: Node): number {
    if (!hoveredNode) return 1
    if (n.id === hoveredNode.id || connectedToHovered.has(n.id)) return 1
    return 0.12
  }

  function linkOpacity(l: Link): number {
    if (!hoveredNode) return 0.35
    if (l.source.id === hoveredNode.id || l.target.id === hoveredNode.id) return 0.9
    return 0.04
  }

  function rebuild(noteList: string[], edgeList: { source: string; target: string }[]): void {
    const known = new Set(noteList)
    const ids = new Set<string>(noteList)
    for (const e of edgeList) { ids.add(e.source); ids.add(e.target) }

    const degree = new Map<string, number>()
    for (const e of edgeList) {
      degree.set(e.source, (degree.get(e.source) ?? 0) + 1)
      degree.set(e.target, (degree.get(e.target) ?? 0) + 1)
    }

    const prev = new Map(nodes.map((n) => [n.id, n]))
    const nodeMap = new Map<string, Node>()
    for (const id of ids) {
      const ex = prev.get(id)
      nodeMap.set(id, ex
        ? { ...ex, exists: known.has(id), degree: degree.get(id) ?? 0, folder: folderOf(id) }
        : { id, exists: known.has(id), degree: degree.get(id) ?? 0, folder: folderOf(id) })
    }
    nodes = Array.from(nodeMap.values())
    links = edgeList
      .filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target))
      .map((e) => ({ source: nodeMap.get(e.source)!, target: nodeMap.get(e.target)! }))

    simulation?.stop()
    simulation = forceSimulation(nodes)
      .force('link', forceLink<Node, Link>(links).id((d) => d.id).distance(110))
      .force('charge', forceManyBody().strength(-300))
      .force('center', forceCenter(width / 2, height / 2).strength(0.05))
      .force('collide', forceCollide((n) => nodeRadius(n as Node) + 14))
      .force('x', forceX(width / 2).strength((n) => (n as Node).degree === 0 ? 0.12 : 0.01))
      .force('y', forceY(height / 2).strength((n) => (n as Node).degree === 0 ? 0.12 : 0.01))
      .on('tick', () => { nodes = nodes; links = links })
  }

  $: if (container) {
    width = container.clientWidth || width
    height = container.clientHeight || height
    if (!vbInitialized) { vb = { x: 0, y: 0, w: width, h: height }; vbInitialized = true }
    rebuild(notes, edges)
  }

  onDestroy(() => simulation?.stop())

  function onNodePointerDown(e: PointerEvent, node: Node): void {
    dragNode = node
    node.fx = node.x; node.fy = node.y
    simulation?.alphaTarget(0.3).restart()
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }

  function onBgPointerDown(e: PointerEvent): void {
    panning = true
    panStart = { x: e.clientX, y: e.clientY, vbX: vb.x, vbY: vb.y }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }

  function onPointerMove(e: PointerEvent): void {
    if (dragNode && container) {
      const rect = container.getBoundingClientRect()
      dragNode.fx = vb.x + ((e.clientX - rect.left) / width) * vb.w
      dragNode.fy = vb.y + ((e.clientY - rect.top) / height) * vb.h
      return
    }
    if (panning) {
      vb = {
        ...vb,
        x: panStart.vbX - ((e.clientX - panStart.x) / width) * vb.w,
        y: panStart.vbY - ((e.clientY - panStart.y) / height) * vb.h,
      }
    }
  }

  function onPointerUp(): void {
    if (dragNode) { dragNode.fx = null; dragNode.fy = null; simulation?.alphaTarget(0); dragNode = null }
    panning = false
  }

  function onWheel(e: WheelEvent): void {
    e.preventDefault()
    if (!container) return
    const rect = container.getBoundingClientRect()
    const mx = e.clientX - rect.left
    const my = e.clientY - rect.top
    const scale = e.deltaY < 0 ? 0.85 : 1.15
    const vx = vb.x + (mx / width) * vb.w
    const vy = vb.y + (my / height) * vb.h
    const newW = Math.max(80, Math.min(8000, vb.w * scale))
    const newH = Math.max(80, Math.min(8000, vb.h * scale))
    vb = { x: vx - (mx / width) * newW, y: vy - (my / height) * newH, w: newW, h: newH }
  }
</script>

<div class="graph-container" bind:this={container}>
  <div class="graph-controls">
    <span class="ctrl-label">深さ: {maxDepth === 0 ? '全て' : maxDepth}</span>
    <input type="range" min="0" max="5" step="1" bind:value={maxDepth} class="depth-slider" />
    <span class="ctrl-hint">ダブルクリックで開く</span>
  </div>

  <svg
    {width}
    {height}
    viewBox="{vb.x} {vb.y} {vb.w} {vb.h}"
    on:wheel={onWheel}
    on:pointermove={onPointerMove}
    on:pointerup={onPointerUp}
    on:pointerleave={onPointerUp}
  >
    <defs>
      <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
        <feGaussianBlur stdDeviation="4" result="blur" />
        <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
      </filter>
      <marker id="arrow" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto">
        <path d="M0,0 L6,3 L0,6 z" class="arrow-head" />
      </marker>
    </defs>

    <rect x={vb.x} y={vb.y} width={vb.w} height={vb.h}
      fill="transparent" class="graph-bg" on:pointerdown={onBgPointerDown} />

    <!-- エッジ -->
    {#each links as link}
      {#if visible(link.source.id) && visible(link.target.id)}
        {@const sx = link.source.x ?? 0}
        {@const sy = link.source.y ?? 0}
        {@const tx = link.target.x ?? 0}
        {@const ty = link.target.y ?? 0}
        {@const cx = (sx + tx) / 2 - (ty - sy) * 0.12}
        {@const cy = (sy + ty) / 2 + (tx - sx) * 0.12}
        <path
          d="M{sx},{sy} Q{cx},{cy} {tx},{ty}"
          class="edge"
          opacity={linkOpacity(link)}
          marker-end="url(#arrow)"
        />
      {/if}
    {/each}

    <!-- ノード -->
    {#each nodes as node (node.id)}
      {#if visible(node.id)}
        {@const r = nodeRadius(node)}
        {@const color = colorFor(node.folder)}
        {@const label = labelOf(node.id)}
        {@const isCurrent = node.id === currentNote}
        {@const lw = label.length * 5.8 + 8}
        <g
          class="node"
          transform="translate({node.x ?? 0},{node.y ?? 0})"
          opacity={nodeOpacity(node)}
          on:pointerdown={(e) => onNodePointerDown(e, node)}
          on:pointerenter={() => { hoveredNode = node }}
          on:pointerleave={() => { hoveredNode = null }}
          on:dblclick={() => dispatch('select', node.id)}
        >
          {#if isCurrent}
            <circle r={r + 5} style="fill:none;stroke:{color};stroke-width:1.5"
              opacity="0.4" filter="url(#glow)" />
            <circle r={r + 2} style="fill:none;stroke:{color};stroke-width:1" opacity="0.6" />
          {/if}
          <circle
            r={r}
            style="fill:{node.exists ? color : 'var(--bg-secondary)'};stroke:{color};stroke-width:{node.exists ? 0 : 1.5};stroke-dasharray:{node.exists ? 'none' : '3 2'}"
          />
          <rect x={-lw / 2} y={-(r + 18)} width={lw} height={12}
            rx="3" fill="var(--bg)" opacity="0.8" />
          <text
            dy={-(r + 8)}
            style="font-size:{Math.max(9, Math.min(11, 9 + node.degree * 0.4))}px;font-weight:{isCurrent ? '600' : '400'};fill:{isCurrent ? color : 'var(--text)'}"
          >{label}</text>
        </g>
      {/if}
    {/each}
  </svg>
</div>

<style>
  .graph-container {
    width: 100%;
    height: 100%;
    position: relative;
  }

  svg { display: block; }

  .graph-bg { cursor: grab; }
  .graph-bg:active { cursor: grabbing; }

  .graph-controls {
    position: absolute;
    bottom: 1rem;
    left: 1rem;
    z-index: 2;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.45rem 0.8rem;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.75rem;
    color: var(--text-dim);
    box-shadow: 0 2px 8px rgba(0,0,0,0.25);
  }

  .ctrl-label { min-width: 4rem; }
  .ctrl-hint { opacity: 0.5; }

  .depth-slider { width: 80px; }

  .edge {
    fill: none;
    stroke: var(--border);
    stroke-width: 1;
    transition: opacity 0.1s;
  }

  .arrow-head {
    fill: var(--border);
    opacity: 0.5;
  }

  .node {
    cursor: pointer;
    transition: opacity 0.12s;
  }

  .node text {
    text-anchor: middle;
    pointer-events: none;
    user-select: none;
  }
</style>
