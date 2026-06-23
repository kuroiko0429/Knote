<script lang="ts">
  import { onDestroy, createEventDispatcher } from 'svelte'
  import {
    forceSimulation,
    forceLink,
    forceManyBody,
    forceCenter,
    forceCollide,
    type Simulation,
    type SimulationNodeDatum,
  } from 'd3-force'

  export let notes: string[]
  export let edges: { source: string; target: string }[]
  export let currentNote: string | null

  interface Node extends SimulationNodeDatum {
    id: string
    exists: boolean
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

  let vb = { x: 0, y: 0, w: 600, h: 600 }
  let vbInitialized = false
  let panning = false
  let panStart = { x: 0, y: 0, vbX: 0, vbY: 0 }

  function rebuild(noteList: string[], edgeList: { source: string; target: string }[]): void {
    const known = new Set(noteList)
    const ids = new Set<string>(noteList)
    for (const e of edgeList) {
      ids.add(e.source)
      ids.add(e.target)
    }

    const prev = new Map(nodes.map((n) => [n.id, n]))
    const nodeMap = new Map<string, Node>()
    for (const id of ids) {
      const existing = prev.get(id)
      nodeMap.set(id, existing ? { ...existing, exists: known.has(id) } : { id, exists: known.has(id) })
    }
    nodes = Array.from(nodeMap.values())
    links = edgeList
      .filter((e) => nodeMap.has(e.source) && nodeMap.has(e.target))
      .map((e) => ({ source: nodeMap.get(e.source)!, target: nodeMap.get(e.target)! }))

    simulation?.stop()
    simulation = forceSimulation(nodes)
      .force(
        'link',
        forceLink<Node, Link>(links)
          .id((d) => d.id)
          .distance(90),
      )
      .force('charge', forceManyBody().strength(-220))
      .force('center', forceCenter(width / 2, height / 2))
      .force('collide', forceCollide(30))
      .on('tick', () => {
        nodes = nodes
        links = links
      })
  }

  $: if (container) {
    width = container.clientWidth || width
    height = container.clientHeight || height
    if (!vbInitialized) {
      vb = { x: 0, y: 0, w: width, h: height }
      vbInitialized = true
    }
    rebuild(notes, edges)
  }

  onDestroy(() => simulation?.stop())

  function onPointerDown(e: PointerEvent, node: Node): void {
    dragNode = node
    node.fx = node.x
    node.fy = node.y
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
      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top
      dragNode.fx = vb.x + (mx / width) * vb.w
      dragNode.fy = vb.y + (my / height) * vb.h
      return
    }
    if (panning) {
      const dxPix = e.clientX - panStart.x
      const dyPix = e.clientY - panStart.y
      vb = {
        ...vb,
        x: panStart.vbX - (dxPix / width) * vb.w,
        y: panStart.vbY - (dyPix / height) * vb.h,
      }
    }
  }

  function onPointerUp(): void {
    if (dragNode) {
      dragNode.fx = null
      dragNode.fy = null
      simulation?.alphaTarget(0)
      dragNode = null
    }
    panning = false
  }

  function onWheel(e: WheelEvent): void {
    e.preventDefault()
    if (!container) return
    const rect = container.getBoundingClientRect()
    const mx = e.clientX - rect.left
    const my = e.clientY - rect.top
    const scale = e.deltaY < 0 ? 0.9 : 1.1
    const vx = vb.x + (mx / width) * vb.w
    const vy = vb.y + (my / height) * vb.h
    const newW = Math.max(80, Math.min(8000, vb.w * scale))
    const newH = Math.max(80, Math.min(8000, vb.h * scale))
    vb = {
      x: vx - (mx / width) * newW,
      y: vy - (my / height) * newH,
      w: newW,
      h: newH,
    }
  }

  function labelOf(id: string): string {
    const i = id.lastIndexOf('/')
    return i === -1 ? id : id.slice(i + 1)
  }
</script>

<div class="graph-container" bind:this={container}>
  <svg
    {width}
    {height}
    viewBox="{vb.x} {vb.y} {vb.w} {vb.h}"
    on:wheel={onWheel}
    on:pointermove={onPointerMove}
    on:pointerup={onPointerUp}
    on:pointerleave={onPointerUp}
  >
    <rect
      x={vb.x}
      y={vb.y}
      width={vb.w}
      height={vb.h}
      fill="transparent"
      class="graph-bg"
      on:pointerdown={onBgPointerDown}
    />
    {#each links as link}
      <line x1={link.source.x} y1={link.source.y} x2={link.target.x} y2={link.target.y} class="edge" />
    {/each}
    {#each nodes as node (node.id)}
      <g
        class="node"
        class:current={node.id === currentNote}
        class:phantom={!node.exists}
        transform="translate({node.x},{node.y})"
        on:pointerdown={(e) => onPointerDown(e, node)}
        on:dblclick={() => dispatch('select', node.id)}
      >
        <circle r={node.id === currentNote ? 8 : 6} />
        <text dy="-10">{labelOf(node.id)}</text>
      </g>
    {/each}
  </svg>
</div>

<style>
  .graph-container {
    width: 100%;
    height: 100%;
  }

  svg {
    display: block;
  }

  .graph-bg {
    cursor: grab;
  }

  .edge {
    stroke: var(--border);
    stroke-width: 1;
  }

  .node circle {
    fill: var(--accent);
    cursor: pointer;
  }

  .node.phantom circle {
    fill: none;
    stroke: var(--accent);
    stroke-width: 1;
    stroke-dasharray: 3 2;
  }

  .node.current circle {
    fill: #ffb86c;
  }

  .node text {
    fill: var(--text);
    font-size: 0.7rem;
    text-anchor: middle;
    pointer-events: none;
    user-select: none;
  }
</style>
