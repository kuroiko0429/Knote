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

  function onPointerMove(e: PointerEvent): void {
    if (!dragNode || !container) return
    const rect = container.getBoundingClientRect()
    dragNode.fx = e.clientX - rect.left
    dragNode.fy = e.clientY - rect.top
  }

  function onPointerUp(): void {
    if (!dragNode) return
    dragNode.fx = null
    dragNode.fy = null
    simulation?.alphaTarget(0)
    dragNode = null
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
    on:pointermove={onPointerMove}
    on:pointerup={onPointerUp}
    on:pointerleave={onPointerUp}
  >
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
