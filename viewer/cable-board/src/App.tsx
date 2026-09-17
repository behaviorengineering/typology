import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Edge,
  type NodeMouseHandler,
  type OnMove,
  type OnNodeDrag,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import PackageNode, { type PackageFlowNode } from './PackageNode'
import { InboundSocket, OutboundJack } from './PlugGlyph'
import {
  applySavedNodePositions,
  loadBoardState,
  navKey,
  resetViewState,
  saveBoardState,
  saveNodePositionsForKey,
  type BoardViewport,
} from './boardPersist'
import { layoutGraph } from './layout'
import { analyzeRisks } from './risks'
import type { PackageGraph } from './types'
import WiringEdge, { applyCableNudges, cableNudges } from './WiringEdge'

const nodeTypes = { package: PackageNode }
const edgeTypes = { wiring: WiringEdge }

type WiringLayer = 'all' | 'imports' | 'types' | 'calls' | 'data'

const wiringLayers: Array<{
  id: WiringLayer
  label: string
  kinds: string[] | null
  note: string
  showRisks: boolean
  sampleKind: string
  sampleKinds: string[]
}> = [
  {
    id: 'all',
    label: 'All',
    kinds: null,
    note: 'full harvest overview',
    showRisks: true,
    sampleKind: 'imports',
    sampleKinds: ['imports', 'fills_dto', 'composes', 'uses_runner', 'serves_server', 'reads_config'],
  },
  {
    id: 'imports',
    label: 'Imports',
    kinds: ['imports'],
    note: 'package dependency wiring only',
    showRisks: true,
    sampleKind: 'imports',
    sampleKinds: ['imports'],
  },
  {
    id: 'types',
    label: 'Types',
    kinds: ['fills_dto', 'composes'],
    note: 'dto and shape wiring',
    showRisks: false,
    sampleKind: 'fills_dto',
    sampleKinds: ['fills_dto', 'composes'],
  },
  {
    id: 'calls',
    label: 'Calls',
    kinds: ['uses_runner', 'serves_server'],
    note: 'execution / invocation wiring',
    showRisks: false,
    sampleKind: 'uses_runner',
    sampleKinds: ['uses_runner', 'serves_server'],
  },
  {
    id: 'data',
    label: 'Data',
    kinds: ['reads_config'],
    note: 'config and data wiring',
    showRisks: false,
    sampleKind: 'reads_config',
    sampleKinds: ['reads_config'],
  },
]

function isWiringLayer(value: string): value is WiringLayer {
  return wiringLayers.some((layer) => layer.id === value)
}

function BoardInner() {
  const saved = useMemo(() => loadBoardState(), [])
  const [graph, setGraph] = useState<PackageGraph | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(saved.selectedId)
  const [selectedLayer, setSelectedLayer] = useState<WiringLayer>(
    isWiringLayer(saved.layer) ? saved.layer : 'all',
  )
  const [legendOpen, setLegendOpen] = useState(saved.legendOpen)
  const [helpOpen, setHelpOpen] = useState(false)
  const [layoutEpoch, setLayoutEpoch] = useState(0)
  const [hoverId, setHoverId] = useState<string | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<PackageFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const toolbarRef = useRef<HTMLDivElement | null>(null)
  const legendToolbarRef = useRef<HTMLDivElement | null>(null)
  const viewportsRef = useRef<Record<string, BoardViewport>>({ ...saved.viewports })
  const restoreViewportOnceRef = useRef(true)
  const { fitView, setViewport, getViewport } = useReactFlow()

  const risks = useMemo(() => (graph ? analyzeRisks(graph) : null), [graph])
  const selectedLayerMeta = wiringLayers.find((layer) => layer.id === selectedLayer)

  useEffect(() => {
    let cancelled = false
    const params = new URLSearchParams(window.location.search)
    const src = (params.get('src') || '/assembly-graph.json').trim() || '/assembly-graph.json'
    fetch(src)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(
            `failed to load cable board JSON from ${src} (${res.status}). Run typology assembly-graph and copy the file to viewer/cable-board/public/assembly-graph.json`,
          )
        }
        return res.json() as Promise<PackageGraph>
      })
      .then((data) => {
        if (!cancelled) setGraph(data)
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    saveBoardState({
      layer: selectedLayer,
      selectedId,
      legendOpen,
      viewports: viewportsRef.current,
      viewport: getViewport(),
    })
  }, [selectedLayer, selectedId, legendOpen, getViewport])

  useEffect(() => {
    if (!graph || !selectedLayerMeta) {
      setNodes([])
      setEdges([])
      return
    }
    let cancelled = false
    const key = navKey(selectedLayer, selectedId)
    layoutGraph(graph, selectedId, {
      kinds: selectedLayerMeta.kinds,
      showRisks: selectedLayerMeta.showRisks,
      risks,
    }).then(({ nodes: nextNodes, edges: nextEdges }) => {
      if (cancelled) return
      if (selectedId && !nextNodes.some((n) => n.id === selectedId)) {
        setSelectedId(null)
        return
      }
      setNodes(applySavedNodePositions(nextNodes, key))
      setEdges(applyCableNudges(nextEdges))
      requestAnimationFrame(() => {
        const remembered = viewportsRef.current[key]
        if (remembered) {
          setViewport(remembered, { duration: restoreViewportOnceRef.current ? 0 : 280 })
          restoreViewportOnceRef.current = false
          return
        }
        fitView({
          padding: selectedId ? 0.34 : 0.28,
          duration: 320,
          maxZoom: selectedId ? 1.35 : 1.15,
        })
        restoreViewportOnceRef.current = false
      })
    })
    return () => {
      cancelled = true
    }
  }, [
    graph,
    risks,
    selectedId,
    selectedLayer,
    selectedLayerMeta,
    layoutEpoch,
    setNodes,
    setEdges,
    fitView,
    setViewport,
  ])

  useEffect(() => {
    if (!legendOpen && !helpOpen) return
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      const inHelp = toolbarRef.current?.contains(target)
      const inLegend = legendToolbarRef.current?.contains(target)
      if (!inHelp && !inLegend) {
        setLegendOpen(false)
        setHelpOpen(false)
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setLegendOpen(false)
        setHelpOpen(false)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [legendOpen, helpOpen])

  const persistViewport = useCallback(() => {
    const vp = getViewport()
    const key = navKey(selectedLayer, selectedId)
    viewportsRef.current = { ...viewportsRef.current, [key]: vp }
    saveBoardState({
      layer: selectedLayer,
      selectedId,
      legendOpen,
      viewport: vp,
      viewports: viewportsRef.current,
    })
  }, [getViewport, selectedLayer, selectedId, legendOpen])

  const onMoveEnd: OnMove = useCallback(() => {
    persistViewport()
  }, [persistViewport])

  const onNodeClick: NodeMouseHandler = useCallback(
    (_event, node) => {
      persistViewport()
      setHoverId(null)
      setSelectedId((prev) => (prev === node.id ? null : node.id))
    },
    [persistViewport],
  )

  const onNodeMouseEnter: NodeMouseHandler = useCallback((_event, node) => {
    setHoverId(node.id)
  }, [])

  const onNodeMouseLeave: NodeMouseHandler = useCallback(() => {
    setHoverId(null)
  }, [])

  const displayNodes = useMemo(() => {
    if (selectedId || !hoverId) return nodes
    return nodes.map((node) => {
      const hot =
        node.id === hoverId ||
        edges.some(
          (e) =>
            (e.source === hoverId && e.target === node.id) ||
            (e.target === hoverId && e.source === node.id),
        )
      return {
        ...node,
        data: { ...node.data, dimmed: !hot },
      }
    })
  }, [nodes, edges, hoverId, selectedId])

  const displayEdges = useMemo((): Edge[] => {
    if (selectedId || !hoverId) return edges
    return edges.map((edge) => {
      const hot = edge.source === hoverId || edge.target === hoverId
      const prevStroke =
        edge.style && typeof edge.style.stroke === 'string' ? edge.style.stroke : '#64748b'
      return {
        ...edge,
        style: {
          ...edge.style,
          opacity: hot ? 1 : 0.08,
          strokeWidth: hot ? 2.4 : 1,
          stroke: hot ? '#1d4ed8' : prevStroke,
        },
        zIndex: hot ? 9 : 0,
        label: hot ? edge.label : undefined,
      }
    })
  }, [edges, hoverId, selectedId])

  const onPaneClick = useCallback(() => {
    persistViewport()
    setSelectedId(null)
    setHoverId(null)
    setLegendOpen(false)
    setHelpOpen(false)
  }, [persistViewport])

  const onLayerClick = useCallback(
    (layer: WiringLayer) => {
      persistViewport()
      setSelectedId(null)
      setSelectedLayer(layer)
    },
    [persistViewport],
  )

  const onNodeDragStop: OnNodeDrag<PackageFlowNode> = useCallback(
    (_event, _node, nextNodes) => {
      const key = navKey(selectedLayer, selectedId)
      saveNodePositionsForKey(
        key,
        nextNodes.map((n) => ({ id: n.id, position: n.position })),
      )
    },
    [selectedLayer, selectedId],
  )

  const onResetView = useCallback(() => {
    const key = navKey(selectedLayer, selectedId)
    const edgeIds = edges.map((e) => e.id)
    const next = resetViewState(key, edgeIds, cableNudges)
    viewportsRef.current = { ...next.viewports }
    restoreViewportOnceRef.current = false
    setLayoutEpoch((n) => n + 1)
    requestAnimationFrame(() => {
      fitView({
        padding: selectedId ? 0.34 : 0.28,
        duration: 320,
        maxZoom: selectedId ? 1.35 : 1.15,
      })
    })
  }, [selectedLayer, selectedId, edges, fitView])

  const summary = useMemo(() => {
    if (!graph || !selectedLayerMeta) return ''
    const base = selectedId
      ? `Focus: ${nodes.length} packages · ${edges.length} cables`
      : `${nodes.length} packages · ${edges.length} cables`
    if (selectedLayerMeta.showRisks && risks) {
      return `${base} · risk: ${risks.summary}`
    }
    return `${base} · ${selectedLayerMeta.note}`
  }, [
    graph,
    risks,
    selectedId,
    selectedLayerMeta?.label,
    selectedLayerMeta?.showRisks,
    selectedLayerMeta?.note,
    nodes.length,
    edges.length,
  ])

  if (error) {
    return <div className="banner banner--err">{error}</div>
  }
  if (!graph) {
    return <div className="banner">Loading cable board…</div>
  }

  return (
    <div className="app">
      <header className="header">
        <div>
          <div className="header__title-row" ref={toolbarRef}>
            <h1>Typology cable board</h1>
            <button
              type="button"
              className={`help-toggle ${helpOpen ? 'help-toggle--open' : ''}`}
              aria-label="How to use this board"
              aria-expanded={helpOpen}
              aria-controls="board-help-panel"
              title="How to use this board"
              onClick={() => {
                setHelpOpen((open) => !open)
                setLegendOpen(false)
              }}
            >
              i
            </button>
            {helpOpen ? (
              <div
                id="board-help-panel"
                className="help-popout"
                role="dialog"
                aria-label="Board help"
              >
                <h2 className="help-popout__title">How to use</h2>
                <section className="help-popout__section">
                  <h3>Layers</h3>
                  <ul>
                    <li>
                      <strong>All</strong> shows the full harvest.
                    </li>
                    <li>
                      <strong>Imports / Types / Calls / Data</strong> each show one edge family.
                    </li>
                    <li>Single-kind layers hide repeated cable labels.</li>
                  </ul>
                </section>
                <section className="help-popout__section">
                  <h3>Explore</h3>
                  <ul>
                    <li>Hover a package to isolate its wiring.</li>
                    <li>Click a package to pin a 1-hop star.</li>
                    <li>Open Legend for plug glyphs and risk marks.</li>
                  </ul>
                </section>
                <section className="help-popout__section">
                  <h3>Remembered in this browser</h3>
                  <ul>
                    <li>Cable nudges, box positions, and the camera are saved per view.</li>
                    <li>
                      <strong>Reset view</strong> clears only the layer and focus you are on.
                    </li>
                  </ul>
                </section>
              </div>
            ) : null}
          </div>
          <p className="header__tagline">
            Interactive cable board for typology <code>assembly-graph.json</code> (imports, roles,
            wrong-way marks).
          </p>
          <div className="toolbar" ref={legendToolbarRef}>
            <div className="layer-switch" role="tablist" aria-label="Wiring layers">
              {wiringLayers.map((layer) => (
                <button
                  key={layer.id}
                  type="button"
                  className={`layer-switch__button ${
                    selectedLayer === layer.id ? 'layer-switch__button--active' : ''
                  }`}
                  aria-pressed={selectedLayer === layer.id}
                  onClick={() => onLayerClick(layer.id)}
                >
                  {layer.label}
                </button>
              ))}
            </div>
            <button
              type="button"
              className={`legend-popout__toggle ${
                legendOpen ? 'legend-popout__toggle--open' : ''
              }`}
              onClick={() => {
                setLegendOpen((open) => !open)
                setHelpOpen(false)
              }}
              aria-expanded={legendOpen}
              aria-controls="layer-legend-panel"
            >
              Legend
            </button>
            <button
              type="button"
              className="board-reset"
              title="Reset box positions, cable nudges, and camera for this view only"
              onClick={onResetView}
            >
              Reset view
            </button>
            {legendOpen ? (
              <div
                id="layer-legend-panel"
                className="legend-popout__panel"
                role="dialog"
                aria-label="Wiring legend"
              >
                <div className="layer-legend">
                  {wiringLayers.map((layer) => (
                    <button
                      key={layer.id}
                      type="button"
                      className={`layer-legend__card ${
                        selectedLayer === layer.id ? 'layer-legend__card--active' : ''
                      }`}
                      onClick={() => {
                        onLayerClick(layer.id)
                        setLegendOpen(false)
                      }}
                    >
                      <div className="layer-legend__diagram" aria-hidden>
                        <span className="layer-legend__box layer-legend__box--src">pkg</span>
                        <OutboundJack size={5} />
                        <span className="layer-legend__cable" />
                        <InboundSocket kind={layer.sampleKind} width={14} height={10} />
                        <span className="layer-legend__box layer-legend__box--dst">pkg</span>
                      </div>
                      <div className="layer-legend__copy">
                        <span className="layer-legend__label">{layer.label}</span>
                        <span className="layer-legend__note">{layer.note}</span>
                        <span className="layer-legend__kinds">
                          {layer.sampleKinds.map((k) => (
                            <code key={k}>{k}</code>
                          ))}
                        </span>
                      </div>
                    </button>
                  ))}
                </div>
                <div className="risk-legend" aria-label="Risk marks">
                  <span className="risk-legend__item risk-legend__item--cycle">cycle</span>
                  <span className="risk-legend__item risk-legend__item--mesh">mesh</span>
                  <span className="risk-legend__item risk-legend__item--wrong">wrong-way</span>
                  <span className="risk-legend__hint">
                    Risk marks appear on All and Imports (cycles, dense middle mesh,
                    inner→outer edges)
                  </span>
                </div>
              </div>
            ) : null}
          </div>
          <p className="header__meta">{summary}</p>
        </div>
      </header>
      <div className="canvas">
        <ReactFlow
          nodes={displayNodes}
          edges={displayEdges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onNodeClick={onNodeClick}
          onNodeMouseEnter={onNodeMouseEnter}
          onNodeMouseLeave={onNodeMouseLeave}
          onNodeDragStop={onNodeDragStop}
          onPaneClick={onPaneClick}
          onMoveEnd={onMoveEnd}
          nodesDraggable
          minZoom={0.2}
          proOptions={{ hideAttribution: true }}
        >
          <Background gap={20} />
          <Controls />
          <MiniMap pannable zoomable position="top-right" />
        </ReactFlow>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <ReactFlowProvider>
      <BoardInner />
    </ReactFlowProvider>
  )
}
