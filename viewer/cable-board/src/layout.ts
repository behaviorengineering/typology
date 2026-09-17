import ELK, { type ElkNode } from 'elkjs/lib/elk.bundled.js'
import type { Edge } from '@xyflow/react'
import type { PackageFlowNode } from './PackageNode'
import { analyzeRisks, type GraphRisks } from './risks'
import type { PackageGraph } from './types'
import { shortLabel } from './types'

const elk = new ELK()

type LayerFilter = {
  kinds: string[] | null
  showRisks: boolean
  risks?: GraphRisks | null
}

function nodeSize(inDegree: number, outDegree: number, inboundCount: number, outboundCount: number) {
  const height = 78 + Math.max(inboundCount, outboundCount, 1) * 10
  const width = 160 + Math.min(inDegree + outDegree, 12) * 6
  return { width, height }
}

function peerCenterY(
  peerId: string,
  pos: Map<string, { x: number; y: number }>,
  sizes: Map<string, { width: number; height: number }>,
): number {
  const p = pos.get(peerId)
  const s = sizes.get(peerId)
  if (!p) return 0
  return p.y + (s?.height ?? 0) / 2
}

function handleY(
  nodeId: string,
  peerId: string,
  side: 'in' | 'out',
  nodes: PackageFlowNode[],
  pos: Map<string, { x: number; y: number }>,
  sizes: Map<string, { width: number; height: number }>,
): number {
  const node = nodes.find((n) => n.id === nodeId)
  const p = pos.get(nodeId)
  const s = sizes.get(nodeId)
  if (!node || !p || !s) return p?.y ?? 0
  const ports = side === 'in' ? node.data.inbound : node.data.outbound
  const index = Math.max(0, ports.findIndex((port) => port.id === peerId))
  const count = Math.max(ports.length, 1)
  return p.y + ((index + 1) / (count + 1)) * s.height
}

/**
 * Match socket order to cable geometry after ELK.
 * Outbound: by target center Y.
 * Inbound: by where the cable leaves the peer (outbound handle Y), not peer center.
 * That way a left-side skip (dashboard→board) does not steal the top socket from a
 * mid-column neighbor (remotegit→board) and cross it.
 */
function reorderPortsForPositions(
  nodes: PackageFlowNode[],
  pos: Map<string, { x: number; y: number }>,
  sizes: Map<string, { width: number; height: number }>,
) {
  for (const n of nodes) {
    n.data.outbound = [...n.data.outbound].sort(
      (a, b) => peerCenterY(a.id, pos, sizes) - peerCenterY(b.id, pos, sizes),
    )
  }
  for (const n of nodes) {
    n.data.inbound = [...n.data.inbound].sort((a, b) => {
      const ya = handleY(a.id, n.id, 'out', nodes, pos, sizes)
      const yb = handleY(b.id, n.id, 'out', nodes, pos, sizes)
      return ya - yb
    })
  }
}

export async function layoutGraph(
  graph: PackageGraph,
  selectedId: string | null,
  layer?: LayerFilter,
): Promise<{ nodes: PackageFlowNode[]; edges: Edge[] }> {
  const activeEdges =
    layer?.kinds === null
      ? graph.edges
      : graph.edges.filter((e) => e.roleKind !== undefined && layer?.kinds?.includes(e.roleKind))

  const risk = layer?.showRisks ? layer.risks ?? analyzeRisks(graph) : emptyRisks()
  const focusIds = selectedId !== null ? focusNeighborIdsFromEdges(activeEdges, selectedId) : null

  const activeNodeIds = new Set<string>()
  for (const edge of activeEdges) {
    activeNodeIds.add(edge.source)
    activeNodeIds.add(edge.target)
  }

  const layoutNodes =
    focusIds !== null
      ? graph.nodes.filter((n) => focusIds.has(n.id))
      : activeNodeIds.size > 0
        ? graph.nodes.filter((n) => activeNodeIds.has(n.id))
        : graph.nodes

  const flowNodes: PackageFlowNode[] = layoutNodes.map((n) => {
    const kindInto = new Map<string, string>()
    const kindOut = new Map<string, string>()
    for (const e of activeEdges) {
      if (e.target === n.id && e.roleKind) kindInto.set(e.source, e.roleKind)
      if (e.source === n.id && e.roleKind) kindOut.set(e.target, e.roleKind)
    }

    let inboundIds = activeEdges.filter((e) => e.target === n.id).map((e) => e.source)
    let outboundIds = activeEdges.filter((e) => e.source === n.id).map((e) => e.target)
    if (focusIds && selectedId) {
      inboundIds = inboundIds.filter(
        (id) => focusIds.has(id) && (n.id === selectedId || id === selectedId),
      )
      outboundIds = outboundIds.filter(
        (id) => focusIds.has(id) && (n.id === selectedId || id === selectedId),
      )
    }
    inboundIds = [...new Set(inboundIds)]
    outboundIds = [...new Set(outboundIds)]
    const inbound = inboundIds.map((id) => ({ id, kind: kindInto.get(id) }))
    const outbound = outboundIds.map((id) => ({ id, kind: kindOut.get(id) }))
    const layerInDegree = inbound.length
    const layerOutDegree = outbound.length
    const layerDegree = layerInDegree + layerOutDegree
    return {
      id: n.id,
      type: 'package',
      position: { x: 0, y: 0 },
      data: {
        path: n.path,
        label: shortLabel(n.path),
        inDegree: layerInDegree,
        outDegree: layerOutDegree,
        isHub: layerDegree >= 4,
        isLeaf: layerOutDegree === 0,
        inbound,
        outbound,
        highlighted: selectedId !== null && n.id === selectedId,
        dimmed: false,
        inCycle: risk.cycleNodeIds.has(n.id),
        inMesh: risk.meshNodeIds.has(n.id),
        wrongWay: risk.wrongWayNodeIds.has(n.id),
        role: n.role ?? '',
        isBoundary: Boolean(n.isBoundary),
        boundaryKind: n.boundaryKind,
        ownerId: n.ownerId,
      },
    }
  })

  const layoutEdges = focusIds
    ? activeEdges.filter(
        (e) =>
          (e.source === selectedId && focusIds.has(e.target)) ||
          (e.target === selectedId && focusIds.has(e.source)),
      )
    : activeEdges

  // One-kind layers (Imports, Data) do not need a label on every cable.
  const singleKindLayer = Array.isArray(layer?.kinds) && layer.kinds.length === 1
  const denseOverview = focusIds === null

  const flowEdges: Edge[] = layoutEdges.map((e) => {
    const focused = selectedId !== null
    const cyclic = risk.cycleEdgeIds.has(e.id)
    const wrong = risk.wrongWayEdgeIds.has(e.id)
    const missingBinding = e.bindingStatus === 'missing'
    let stroke = '#64748b'
    let width = denseOverview ? 1.1 : 1.25
    let dash: string | undefined
    if (cyclic) {
      stroke = '#b91c1c'
      width = 2.75
    } else if (wrong) {
      stroke = '#c026d3'
      width = 2.5
    } else if (missingBinding) {
      stroke = '#d97706'
      width = 2.25
      dash = '6 4'
    } else if (focused) {
      stroke = '#1d4ed8'
      width = 2.5
    }
    const label =
      wrong
        ? e.roleKind || 'wrong-way'
        : missingBinding
          ? 'missing binding'
          : singleKindLayer
            ? undefined
            : e.roleKind || undefined
    return {
      id: e.id,
      source: e.source,
      target: e.target,
      sourceHandle: `out-${e.target}`,
      targetHandle: `in-${e.source}`,
      type: 'wiring',
      animated: focused || cyclic || wrong || missingBinding,
      label,
      data: {
        bowX: 0,
        bowY: 0,
        autoBowX: 0,
        autoBowY: 0,
        curvature: denseOverview ? 0.2 : 0.25,
      },
      style: {
        stroke,
        strokeWidth: width,
        opacity: 1,
        ...(dash ? { strokeDasharray: dash } : {}),
      },
      zIndex: focused || cyclic || wrong || missingBinding ? 10 : 1,
      title: e.wrongWayReason || (missingBinding ? 'undeclared cross-boundary import' : e.roleKind) || undefined,
    }
  })

  const sizes = new Map<string, { width: number; height: number }>()
  const elkGraph: ElkNode = {
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.spacing.nodeNode': focusIds ? '168' : denseOverview ? '80' : '56',
      'elk.layered.spacing.nodeNodeBetweenLayers': focusIds
        ? '320'
        : denseOverview
          ? '160'
          : '96',
      'elk.spacing.edgeNode': focusIds ? '36' : denseOverview ? '28' : '18',
      'elk.layered.spacing.edgeNodeBetweenLayers': focusIds
        ? '48'
        : denseOverview
          ? '40'
          : '24',
      'elk.spacing.edgeEdge': denseOverview ? '18' : '12',
      'elk.layered.spacing.edgeEdgeBetweenLayers': denseOverview ? '24' : '14',
      'elk.spacing.portPort': focusIds ? '18' : '14',
      'elk.padding': focusIds ? '[24,24,24,24]' : '[16,16,16,16]',
      'elk.edgeRouting': 'SPLINES',
      'elk.portConstraints': 'FIXED_SIDE',
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'elk.layered.crossingMinimization.semiInteractive': 'true',
      'elk.layered.thoroughness': denseOverview ? '40' : '20',
      'elk.layered.cycleBreaking.strategy': 'GREEDY',
    },
    children: flowNodes.map((n) => {
      const size = nodeSize(
        n.data.inDegree,
        n.data.outDegree,
        n.data.inbound.length,
        n.data.outbound.length,
      )
      sizes.set(n.id, size)
      return {
        id: n.id,
        width: size.width,
        height: size.height,
        ports: [
          ...n.data.inbound.map((peer, i) => ({
            id: `in-${peer.id}`,
            properties: { 'port.side': 'WEST', 'port.index': String(i) },
          })),
          ...n.data.outbound.map((peer, i) => ({
            id: `out-${peer.id}`,
            properties: { 'port.side': 'EAST', 'port.index': String(i) },
          })),
        ],
      }
    }),
    edges: flowEdges.map((e) => ({
      id: e.id,
      sources: [e.source],
      targets: [e.target],
      sourcePort: e.sourceHandle ?? undefined,
      targetPort: e.targetHandle ?? undefined,
    })),
  }

  if (selectedId) {
    const selected = flowNodes.find((n) => n.id === selectedId)
    if (selected) {
      const inboundIds = selected.data.inbound.map((p) => p.id)
      const outboundIds = selected.data.outbound.map((p) => p.id)
      for (const child of elkGraph.children ?? []) {
        if (child.id === selectedId) continue
        const isImporter = inboundIds.includes(child.id)
        const isImport = outboundIds.includes(child.id)
        if (isImporter && !isImport) {
          child.layoutOptions = { 'elk.layered.layering.layerConstraint': 'FIRST' }
        } else if (isImport && !isImporter) {
          child.layoutOptions = { 'elk.layered.layering.layerConstraint': 'LAST' }
        }
      }
    }
  }

  const laid = await elk.layout(elkGraph)
  const pos = new Map<string, { x: number; y: number }>()
  for (const child of laid.children ?? []) {
    pos.set(child.id, { x: child.x ?? 0, y: child.y ?? 0 })
  }

  // Reorder sockets to peer Y, then one FIXED_ORDER pass so ELK respects that order.
  reorderPortsForPositions(flowNodes, pos, sizes)

  const constraintById = new Map(
    (elkGraph.children ?? []).map((c) => [c.id, c.layoutOptions] as const),
  )

  const elkGraphOrdered: ElkNode = {
    ...elkGraph,
    layoutOptions: {
      ...elkGraph.layoutOptions,
      'elk.portConstraints': 'FIXED_ORDER',
    },
    children: flowNodes.map((n) => {
      const size =
        sizes.get(n.id) ??
        nodeSize(n.data.inDegree, n.data.outDegree, n.data.inbound.length, n.data.outbound.length)
      return {
        id: n.id,
        width: size.width,
        height: size.height,
        layoutOptions: constraintById.get(n.id),
        ports: [
          ...n.data.inbound.map((peer, i) => ({
            id: `in-${peer.id}`,
            properties: { 'port.side': 'WEST', 'port.index': String(i) },
          })),
          ...n.data.outbound.map((peer, i) => ({
            id: `out-${peer.id}`,
            properties: { 'port.side': 'EAST', 'port.index': String(i) },
          })),
        ],
      }
    }),
    edges: elkGraph.edges,
  }

  const laid2 = await elk.layout(elkGraphOrdered)
  const pos2 = new Map<string, { x: number; y: number }>()
  for (const child of laid2.children ?? []) {
    pos2.set(child.id, { x: child.x ?? 0, y: child.y ?? 0 })
  }

  reorderPortsForPositions(flowNodes, pos2, sizes)

  // Only nudge a sink when a skip cable would tunnel through another package box.
  const finalPos = clearSkipCableOverlaps(flowNodes, pos2, sizes, layoutEdges)
  reorderPortsForPositions(flowNodes, finalPos, sizes)

  const nodes: PackageFlowNode[] = flowNodes.map((n) => ({
    ...n,
    position: finalPos.get(n.id) ?? pos2.get(n.id) ?? pos.get(n.id) ?? n.position,
  }))

  annotateBowEdges(flowEdges, flowNodes, finalPos, sizes)

  return { nodes, edges: flowEdges }
}

/**
 * Only cables that tunnel through another package or share a crowded socket:
 * switch to a bowed bezier (pull the wire out), not label tricks.
 */
function annotateBowEdges(
  edges: Edge[],
  nodes: PackageFlowNode[],
  pos: Map<string, { x: number; y: number }>,
  sizes: Map<string, { width: number; height: number }>,
) {
  const byTarget = new Map<string, Edge[]>()
  for (const edge of edges) {
    const tList = byTarget.get(edge.target) ?? []
    tList.push(edge)
    byTarget.set(edge.target, tList)
  }

  for (const [, group] of byTarget) {
    if (group.length > 1) {
      group.sort((a, b) => {
        const ya = handleY(a.source, a.target, 'out', nodes, pos, sizes)
        const yb = handleY(b.source, b.target, 'out', nodes, pos, sizes)
        return ya - yb
      })
    }
  }

  for (const edge of edges) {
    const sPos = pos.get(edge.source)
    const tPos = pos.get(edge.target)
    const sSize = sizes.get(edge.source)
    const tSize = sizes.get(edge.target)
    if (!sPos || !tPos || !sSize || !tSize) continue

    const y0 = handleY(edge.source, edge.target, 'out', nodes, pos, sizes)
    const y1 = handleY(edge.target, edge.source, 'in', nodes, pos, sizes)
    const x0 = sPos.x + sSize.width
    const x1 = tPos.x
    const midY = (y0 + y1) / 2

    let bowY = 0
    let needsBow = false
    let blocked = false

    for (const other of nodes) {
      if (other.id === edge.source || other.id === edge.target) continue
      const oPos = pos.get(other.id)
      const oSize = sizes.get(other.id)
      if (!oPos || !oSize) continue
      const pad = 12
      const spansX = oPos.x + oSize.width > x0 && oPos.x < x1
      const spansY =
        oPos.y + oSize.height > Math.min(y0, y1) - pad && oPos.y < Math.max(y0, y1) + pad
      if (!spansX || !spansY || x1 - x0 <= oSize.width) continue

      blocked = true
      const oTop = oPos.y
      const oBottom = oPos.y + oSize.height
      const oCy = oPos.y + oSize.height / 2
      const tCy = tPos.y + tSize.height / 2
      const gap = 44
      // Prefer the side toward the target: dashboard→board pulls under remotegit.
      if (tCy >= oCy) {
        bowY += Math.max(110, oBottom - midY + gap)
      } else {
        bowY -= Math.max(110, midY - oTop + gap)
      }
      needsBow = true
    }

    const siblings = byTarget.get(edge.target) ?? [edge]
    // Fan-in spread only when the wire is not rerouting around a package (skip stays one big bow).
    if (!blocked && siblings.length > 1 && siblings.length <= 4) {
      const index = Math.max(0, siblings.findIndex((s) => s.id === edge.id))
      const spread = 56
      const lane = (index - (siblings.length - 1) / 2) * spread
      bowY += lane
      needsBow = true
    }

    if (!needsBow || Math.abs(bowY) < 8) continue

    edge.type = 'wiring'
    edge.data = {
      bowX: 0,
      bowY,
      autoBowX: 0,
      autoBowY: bowY,
      curvature: blocked ? 0.5 : 0.36,
    }
  }
}

/** Nudge a target down when the cable band intersects an intermediate package. */
function clearSkipCableOverlaps(
  nodes: PackageFlowNode[],
  positions: Map<string, { x: number; y: number }>,
  sizes: Map<string, { width: number; height: number }>,
  edges: GraphEdgeLike[],
): Map<string, { x: number; y: number }> {
  const pos = new Map(positions)
  // Wider clearance in focus so 1-hop skips stay readable without reshaping the star.
  const focused = nodes.some((n) => n.data.highlighted)
  const gap = focused ? 56 : 40
  const pad = focused ? 14 : 10

  for (let round = 0; round < 3; round++) {
    let moved = false
    for (const edge of edges) {
      const sPos = pos.get(edge.source)
      const sSize = sizes.get(edge.source)
      if (!sPos || !sSize) continue

      let tPos = pos.get(edge.target)
      if (!tPos) continue

      const x0 = sPos.x + sSize.width
      const x1 = tPos.x
      if (x1 <= x0 + 24) continue

      const y0 = handleY(edge.source, edge.target, 'out', nodes, pos, sizes)
      const y1 = handleY(edge.target, edge.source, 'in', nodes, pos, sizes)
      const bandTop = Math.min(y0, y1) - pad
      const bandBottom = Math.max(y0, y1) + pad

      for (const other of nodes) {
        if (other.id === edge.source || other.id === edge.target) continue
        const oPos = pos.get(other.id)
        const oSize = sizes.get(other.id)
        if (!oPos || !oSize) continue

        const oRight = oPos.x + oSize.width
        const overlapsX = oRight > x0 && oPos.x < x1
        const overlapsY = oPos.y + oSize.height > bandTop && oPos.y < bandBottom
        if (!overlapsX || !overlapsY) continue

        tPos = pos.get(edge.target) ?? tPos
        const nextY = Math.max(tPos.y, oPos.y + oSize.height + gap)
        if (nextY > tPos.y) {
          pos.set(edge.target, { x: tPos.x, y: nextY })
          moved = true
        }
      }
    }
    if (!moved) break
  }

  return pos
}

function focusNeighborIdsFromEdges(edges: GraphEdgeLike[], selectedId: string): Set<string> {
  const ids = new Set<string>([selectedId])
  for (const e of edges) {
    if (e.source === selectedId) ids.add(e.target)
    if (e.target === selectedId) ids.add(e.source)
  }
  return ids
}

function emptyRisks(): GraphRisks {
  return {
    cycleNodeIds: new Set(),
    cycleEdgeIds: new Set(),
    meshNodeIds: new Set(),
    wrongWayEdgeIds: new Set(),
    wrongWayNodeIds: new Set(),
    cycleGroups: [],
    summary: 'no risk marks',
  }
}

type GraphEdgeLike = {
  source: string
  target: string
}
