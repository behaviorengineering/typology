import type { GraphEdge, GraphNode, PackageGraph } from './types'

/**
 * Composition overlay on slice boards.
 * - inside: owned packages only
 * - external_dependents: stubs that depend on this slice (in-stubs)
 * - depends_on_others: stubs this slice depends on (out-stubs)
 */
export type CompositionScope = 'inside' | 'external_dependents' | 'depends_on_others'

/** How a boundary stub relates to owned packages. */
export type StubDirection = 'in' | 'out' | 'both'

export function graphHasBoundaries(graph: PackageGraph): boolean {
  return graph.nodes.some((n) => Boolean(n.isBoundary))
}

export function ownedNodeIds(graph: PackageGraph): Set<string> {
  return new Set(graph.nodes.filter((n) => !n.isBoundary).map((n) => n.id))
}

export function stubDirectionFor(
  nodeId: string,
  graph: PackageGraph,
  owned: Set<string> = ownedNodeIds(graph),
): StubDirection | null {
  const node = graph.nodes.find((n) => n.id === nodeId)
  if (!node?.isBoundary) return null
  let asSource = false
  let asTarget = false
  for (const e of graph.edges) {
    if (e.source === nodeId && owned.has(e.target)) asSource = true
    if (e.target === nodeId && owned.has(e.source)) asTarget = true
  }
  if (asSource && asTarget) return 'both'
  if (asSource) return 'in'
  if (asTarget) return 'out'
  return null
}

function crossBoundaryEdge(edge: GraphEdge, owned: Set<string>): boolean {
  const srcOwned = owned.has(edge.source)
  const tgtOwned = owned.has(edge.target)
  return srcOwned !== tgtOwned
}

function edgeMatchesScope(edge: GraphEdge, owned: Set<string>, scope: CompositionScope): boolean {
  const fromOwned = owned.has(edge.source)
  const toOwned = owned.has(edge.target)
  if (scope === 'external_dependents') {
    // stub → owned
    return !fromOwned && toOwned
  }
  if (scope === 'depends_on_others') {
    // owned → stub
    return fromOwned && !toOwned
  }
  return false
}

function stubMatchesScope(dir: StubDirection | null, scope: CompositionScope): boolean {
  if (dir === null) return false
  if (scope === 'external_dependents') return dir === 'in' || dir === 'both'
  if (scope === 'depends_on_others') return dir === 'out' || dir === 'both'
  return false
}

/**
 * Filter graph edges/nodes for composition scope overlays.
 * Layer kind filtering still happens in layoutGraph.
 * Non-slice boards (no boundaries) are returned unchanged.
 */
export function filterGraphByScope(
  graph: PackageGraph,
  scope: CompositionScope,
): { nodes: GraphNode[]; edges: GraphEdge[] } {
  if (!graphHasBoundaries(graph)) {
    return { nodes: graph.nodes, edges: graph.edges }
  }

  const owned = ownedNodeIds(graph)

  if (scope === 'inside') {
    const edges = graph.edges.filter((e) => owned.has(e.source) && owned.has(e.target))
    const ids = new Set<string>()
    for (const e of edges) {
      ids.add(e.source)
      ids.add(e.target)
    }
    for (const n of graph.nodes) {
      if (!n.isBoundary) ids.add(n.id)
    }
    return {
      nodes: graph.nodes.filter((n) => ids.has(n.id)),
      edges,
    }
  }

  const edges = graph.edges.filter(
    (e) => crossBoundaryEdge(e, owned) && edgeMatchesScope(e, owned, scope),
  )
  const ids = new Set<string>()
  for (const e of edges) {
    ids.add(e.source)
    ids.add(e.target)
  }
  for (const n of graph.nodes) {
    if (!n.isBoundary) continue
    if (!ids.has(n.id)) continue
    const dir = stubDirectionFor(n.id, graph, owned)
    if (!stubMatchesScope(dir, scope)) ids.delete(n.id)
  }
  const keptEdges = edges.filter((e) => ids.has(e.source) && ids.has(e.target))
  return {
    nodes: graph.nodes.filter((n) => ids.has(n.id)),
    edges: keptEdges,
  }
}

export function stubMetaLabel(dir: StubDirection | null): string {
  switch (dir) {
    case 'in':
      return 'external dependent · they depend on this slice'
    case 'out':
      return 'dependency · this slice depends on them'
    case 'both':
      return 'stub · both directions'
    default:
      return 'boundary stub'
  }
}
