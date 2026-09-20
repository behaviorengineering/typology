import type { PackageGraph } from './types'
import { pathToId } from './types'

export type GraphRisks = {
  /** Node ids that participate in an import cycle (SCC size >= 2). */
  cycleNodeIds: Set<string>
  /** Edge ids that lie on a cycle between cycle members. */
  cycleEdgeIds: Set<string>
  /** Non-entrypoint, non-leaf packages with dense mutual wiring. */
  meshNodeIds: Set<string>
  /** Edges that invert role layers (inner imports outer / leaf pulls delivery). */
  wrongWayEdgeIds: Set<string>
  /** Nodes that emit or receive a wrong-way edge. */
  wrongWayNodeIds: Set<string>
  cycleGroups: string[][]
  summary: string
}

/**
 * Architecture risk marks: cycles + mesh from topology; wrong-way from role layers
 * when present on assembly-graph.json.
 */
export function analyzeRisks(graph: PackageGraph): GraphRisks {
  const adj = new Map<string, string[]>()
  for (const n of graph.nodes) {
    adj.set(n.id, n.imports.map(pathToId))
  }

  const sccs = tarjanSCC(adj)
  const cycleGroups = sccs.filter((g) => g.length >= 2)
  const cycleNodeIds = new Set<string>()
  for (const g of cycleGroups) {
    for (const id of g) cycleNodeIds.add(id)
  }

  const cycleEdgeIds = new Set<string>()
  for (const e of graph.edges) {
    if (cycleNodeIds.has(e.source) && cycleNodeIds.has(e.target)) {
      if (sameComponent(cycleGroups, e.source, e.target)) {
        cycleEdgeIds.add(e.id)
      }
    }
  }

  const entrypointIds = new Set(
    graph.nodes
      .filter((n) => n.path.startsWith('cmd/') || (n.inDegree === 0 && n.outDegree > 0))
      .map((n) => n.id),
  )
  const leafIds = new Set(graph.nodes.filter((n) => n.outDegree === 0).map((n) => n.id))

  const middle = graph.nodes.filter(
    (n) => !entrypointIds.has(n.id) && !leafIds.has(n.id) && n.inDegree > 0 && n.outDegree > 0,
  )
  const middleIds = new Set(middle.map((n) => n.id))

  let middleEdgeCount = 0
  const middleDegree = new Map<string, number>()
  for (const id of middleIds) middleDegree.set(id, 0)
  for (const e of graph.edges) {
    if (middleIds.has(e.source) && middleIds.has(e.target)) {
      middleEdgeCount++
      middleDegree.set(e.source, (middleDegree.get(e.source) ?? 0) + 1)
      middleDegree.set(e.target, (middleDegree.get(e.target) ?? 0) + 1)
    }
  }

  const n = middleIds.size
  const possible = n > 1 ? n * (n - 1) : 0
  const density = possible > 0 ? middleEdgeCount / possible : 0

  const meshNodeIds = new Set<string>()
  const denseGraph = n >= 3 && density >= 0.35
  for (const id of middleIds) {
    const deg = middleDegree.get(id) ?? 0
    if (denseGraph || deg >= 3) {
      meshNodeIds.add(id)
    }
  }

  const wrongWayEdgeIds = new Set<string>()
  const wrongWayNodeIds = new Set<string>()
  for (const e of graph.edges) {
    if (!e.wrongWay) continue
    wrongWayEdgeIds.add(e.id)
    wrongWayNodeIds.add(e.source)
    wrongWayNodeIds.add(e.target)
  }

  const cyclePart =
    cycleGroups.length === 0
      ? 'no import cycles'
      : `${cycleGroups.length} cycle group(s) · ${cycleNodeIds.size} packages`
  const meshPart =
    meshNodeIds.size === 0
      ? 'no dense middle mesh'
      : `${meshNodeIds.size} dense-middle package(s)` +
        (denseGraph ? ` (middle density ${(density * 100).toFixed(0)}%)` : '')
  const wrongPart =
    wrongWayEdgeIds.size === 0
      ? 'no wrong-way role edges'
      : `${wrongWayEdgeIds.size} wrong-way edge(s)`

  return {
    cycleNodeIds,
    cycleEdgeIds,
    meshNodeIds,
    wrongWayEdgeIds,
    wrongWayNodeIds,
    cycleGroups,
    summary: `${cyclePart} · ${meshPart} · ${wrongPart}`,
  }
}

function sameComponent(groups: string[][], a: string, b: string): boolean {
  for (const g of groups) {
    if (g.includes(a) && g.includes(b)) return true
  }
  return false
}

function tarjanSCC(adj: Map<string, string[]>): string[][] {
  let index = 0
  const stack: string[] = []
  const onStack = new Set<string>()
  const indices = new Map<string, number>()
  const lowlink = new Map<string, number>()
  const components: string[][] = []

  function strongconnect(v: string) {
    indices.set(v, index)
    lowlink.set(v, index)
    index++
    stack.push(v)
    onStack.add(v)

    for (const w of adj.get(v) ?? []) {
      if (!indices.has(w)) {
        strongconnect(w)
        lowlink.set(v, Math.min(lowlink.get(v)!, lowlink.get(w)!))
      } else if (onStack.has(w)) {
        lowlink.set(v, Math.min(lowlink.get(v)!, indices.get(w)!))
      }
    }

    if (lowlink.get(v) === indices.get(v)) {
      const comp: string[] = []
      for (;;) {
        const w = stack.pop()!
        onStack.delete(w)
        comp.push(w)
        if (w === v) break
      }
      comp.sort()
      components.push(comp)
    }
  }

  const nodes = [...adj.keys()].sort()
  for (const v of nodes) {
    if (!indices.has(v)) strongconnect(v)
  }
  return components
}
