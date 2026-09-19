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
import { MarksLegend } from './MarksLegend'
import PackageNode, { type PackageFlowNode } from './PackageNode'
import {
  applySavedNodePositions,
  loadBoardState,
  navKey,
  resetViewState,
  saveBoardState,
  saveNodePositionsForKey,
  setActiveBoardId,
  type BoardViewport,
} from './boardPersist'
import {
  boardRepoKey,
  defaultBoardId,
  fetchBoardsManifest,
  filterBoardsByRepo,
  groupBoardsByRepo,
  LEGACY_BOARD_PATH,
  type BoardsManifest,
} from './boards'
import { layoutGraph, separateOverlappingNodes } from './layout'
import { analyzeRisks } from './risks'
import {
  graphHasBoundaries,
  type CompositionScope,
} from './scope'
import type { PackageGraph } from './types'
import WiringEdge, { applyCableNudges, cableNudges, refreshCableNudges } from './WiringEdge'

const nodeTypes = { package: PackageNode }
const edgeTypes = { wiring: WiringEdge }

type WiringLayer = 'imports' | 'types' | 'calls' | 'data'

const wiringLayers: Array<{
  id: WiringLayer
  label: string
  kinds: string[]
  note: string
  showRisks: boolean
  sampleKind: string
  sampleKinds: string[]
}> = [
  {
    id: 'imports',
    label: 'Imports',
    kinds: ['imports'],
    note: 'Who imports whom: ordinary package dependency wiring.',
    showRisks: true,
    sampleKind: 'imports',
    sampleKinds: ['imports'],
  },
  {
    id: 'types',
    label: 'Types',
    kinds: ['fills_dto', 'composes'],
    note: 'Shape wiring: filling DTOs and composing aggregators.',
    showRisks: false,
    sampleKind: 'fills_dto',
    sampleKinds: ['fills_dto', 'composes'],
  },
  {
    id: 'calls',
    label: 'Calls',
    kinds: ['uses_runner', 'serves_server'],
    note: 'Execution wiring: runners and HTTP/server surfaces.',
    showRisks: false,
    sampleKind: 'uses_runner',
    sampleKinds: ['uses_runner', 'serves_server'],
  },
  {
    id: 'data',
    label: 'Data',
    kinds: ['reads_config'],
    note: 'Config and settings packages other code reads.',
    showRisks: false,
    sampleKind: 'reads_config',
    sampleKinds: ['reads_config'],
  },
]

function isWiringLayer(value: string): value is WiringLayer {
  return wiringLayers.some((layer) => layer.id === value)
}

function BoardInner() {
  const [graph, setGraph] = useState<PackageGraph | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [boards, setBoards] = useState<BoardsManifest | null>(null)
  const [boardId, setBoardId] = useState<string | null>(null)
  const [boardLabel, setBoardLabel] = useState<string>('')
  const [boardRepo, setBoardRepo] = useState<string>('')
  const [repoFilter, setRepoFilter] = useState<string>('')
  const [graphSrc, setGraphSrc] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedLayer, setSelectedLayer] = useState<WiringLayer>('imports')
  const [compositionScope, setCompositionScope] = useState<CompositionScope>('inside')
  const [legendOpen, setLegendOpen] = useState(false)
  const [helpOpen, setHelpOpen] = useState(false)
  const [layoutEpoch, setLayoutEpoch] = useState(0)
  const [hoverId, setHoverId] = useState<string | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<PackageFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const toolbarRef = useRef<HTMLDivElement | null>(null)
  const legendToolbarRef = useRef<HTMLDivElement | null>(null)
  const viewportsRef = useRef<Record<string, BoardViewport>>({})
  const restoreViewportOnceRef = useRef(true)
  const { fitView, setViewport, getViewport } = useReactFlow()

  const risks = useMemo(() => (graph ? analyzeRisks(graph) : null), [graph])
  const selectedLayerMeta = wiringLayers.find((layer) => layer.id === selectedLayer)
  const showScopeControls = Boolean(graph && graphHasBoundaries(graph))

  const applyBoard = useCallback((id: string, label: string, src: string, repo?: string) => {
    setActiveBoardId(id)
    refreshCableNudges()
    const saved = loadBoardState()
    viewportsRef.current = { ...saved.viewports }
    restoreViewportOnceRef.current = true
    setSelectedId(saved.selectedId)
    setSelectedLayer(isWiringLayer(saved.layer) ? saved.layer : 'imports')
    setCompositionScope('inside')
    setLegendOpen(saved.legendOpen)
    setHoverId(null)
    setBoardId(id)
    setBoardLabel(label)
    setBoardRepo((repo || '').trim())
    setGraph(null)
    setError(null)
    setGraphSrc(src)
  }, [])

  useEffect(() => {
    let cancelled = false
    async function resolveBoard() {
      const params = new URLSearchParams(window.location.search)
      const srcOverride = (params.get('src') || '').trim()
      const requestedBoard = (params.get('board') || '').trim()
      const requestedRepo = (params.get('repo') || '').trim()
      const manifest = await fetchBoardsManifest()
      if (cancelled) return
      if (manifest) setBoards(manifest)
      if (requestedRepo) setRepoFilter(requestedRepo)
      if (srcOverride !== '') {
        applyBoard('custom', srcOverride, srcOverride)
        return
      }
      if (!manifest) {
        applyBoard('default', 'Default board', LEGACY_BOARD_PATH)
        return
      }
      if (requestedBoard !== '') {
        const entry = manifest.boards.find((board) => board.id === requestedBoard)
        if (!entry) {
          setError(
            `unknown cable board "${requestedBoard}". Available boards: ${manifest.boards
              .map((board) => board.id)
              .join(', ')}`,
          )
          return
        }
        applyBoard(entry.id, entry.label, entry.graph, entry.repo)
        return
      }
      const visible = filterBoardsByRepo(manifest.boards, requestedRepo || null)
      if (visible.length === 0) {
        setError(
          requestedRepo
            ? `no cable boards for repo "${requestedRepo}"`
            : 'boards manifest is empty',
        )
        return
      }
      const preferredId = defaultBoardId(manifest)
      const fallback =
        visible.find((board) => board.id === preferredId) ?? visible[0]
      applyBoard(fallback.id, fallback.label, fallback.graph, fallback.repo)
    }
    void resolveBoard()
    return () => {
      cancelled = true
    }
  }, [applyBoard])

  const switcherBoards = useMemo(() => {
    if (!boards) return []
    return filterBoardsByRepo(boards.boards, repoFilter || null)
  }, [boards, repoFilter])

  const switcherGroups = useMemo(
    () => groupBoardsByRepo(switcherBoards),
    [switcherBoards],
  )

  const repoOptions = useMemo(() => {
    if (!boards) return []
    const seen = new Set<string>()
    const out: string[] = []
    for (const board of boards.boards) {
      const key = boardRepoKey(board)
      if (key === 'ungrouped' || seen.has(key)) continue
      seen.add(key)
      out.push(key)
    }
    return out
  }, [boards])

  useEffect(() => {
    if (!graphSrc) return
    let cancelled = false
    fetch(graphSrc)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(
            `failed to load cable board JSON from ${graphSrc} (${res.status}). Run typology boards register REPO BOARD_ID --viewer viewer/cable-board/public`,
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
  }, [graphSrc])

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
    const key = navKey(selectedLayer, selectedId, compositionScope)
    layoutGraph(graph, selectedId, {
      kinds: selectedLayerMeta.kinds,
      showRisks: selectedLayerMeta.showRisks,
      risks,
      scope: compositionScope,
    }).then(({ nodes: nextNodes, edges: nextEdges }) => {
      if (cancelled) return
      if (selectedId && !nextNodes.some((n) => n.id === selectedId)) {
        setSelectedId(null)
        return
      }
      // Saved positions can stack cards; separate again after restore.
      setNodes(separateOverlappingNodes(applySavedNodePositions(nextNodes, key)))
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
    compositionScope,
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
    const key = navKey(selectedLayer, selectedId, compositionScope)
    viewportsRef.current = { ...viewportsRef.current, [key]: vp }
    saveBoardState({
      layer: selectedLayer,
      selectedId,
      legendOpen,
      viewport: vp,
      viewports: viewportsRef.current,
    })
  }, [getViewport, selectedLayer, selectedId, compositionScope, legendOpen])

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

  const onScopeClick = useCallback(
    (scope: CompositionScope) => {
      persistViewport()
      setSelectedId(null)
      setCompositionScope(scope)
    },
    [persistViewport],
  )

  const onBoardChange = useCallback(
    (nextId: string) => {
      if (!boards) return
      const entry = boards.boards.find((board) => board.id === nextId)
      if (!entry || entry.id === boardId) return
      const vp = getViewport()
      viewportsRef.current = {
        ...viewportsRef.current,
        [navKey(selectedLayer, selectedId, compositionScope)]: vp,
      }
      saveBoardState({
        layer: selectedLayer,
        selectedId,
        legendOpen,
        viewport: vp,
        viewports: viewportsRef.current,
      })
      const url = new URL(window.location.href)
      url.searchParams.delete('src')
      url.searchParams.set('board', entry.id)
      if (entry.repo) {
        url.searchParams.set('repo', entry.repo)
      }
      window.history.replaceState(null, '', url.toString())
      applyBoard(entry.id, entry.label, entry.graph, entry.repo)
    },
    [
      boards,
      boardId,
      getViewport,
      selectedLayer,
      selectedId,
      compositionScope,
      legendOpen,
      applyBoard,
    ],
  )

  const onRepoFilterChange = useCallback(
    (nextRepo: string) => {
      setRepoFilter(nextRepo)
      const url = new URL(window.location.href)
      if (nextRepo) {
        url.searchParams.set('repo', nextRepo)
      } else {
        url.searchParams.delete('repo')
      }
      window.history.replaceState(null, '', url.toString())
      if (!boards) return
      const visible = filterBoardsByRepo(boards.boards, nextRepo || null)
      if (visible.length === 0) return
      if (boardId && visible.some((board) => board.id === boardId)) return
      const entry = visible[0]
      url.searchParams.set('board', entry.id)
      window.history.replaceState(null, '', url.toString())
      applyBoard(entry.id, entry.label, entry.graph, entry.repo)
    },
    [boards, boardId, applyBoard],
  )

  const onNodeDragStop: OnNodeDrag<PackageFlowNode> = useCallback(
    (_event, _node, nextNodes) => {
      const key = navKey(selectedLayer, selectedId, compositionScope)
      const separated = separateOverlappingNodes(nextNodes as PackageFlowNode[])
      setNodes(separated)
      saveNodePositionsForKey(
        key,
        separated.map((n) => ({ id: n.id, position: n.position })),
      )
    },
    [selectedLayer, selectedId, compositionScope, setNodes],
  )

  const onResetView = useCallback(() => {
    const key = navKey(selectedLayer, selectedId, compositionScope)
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
  }, [selectedLayer, selectedId, compositionScope, edges, fitView])

  const summary = useMemo(() => {
    if (!graph || !selectedLayerMeta) return ''
    const base = selectedId
      ? `Focus: ${nodes.length} packages · ${edges.length} cables`
      : `${nodes.length} packages · ${edges.length} cables`
    if (!showScopeControls) {
      if (selectedLayerMeta.showRisks && risks) {
        return `${base} · risk: ${risks.summary}`
      }
      return `${base} · ${selectedLayerMeta.note}`
    }
    const scopeNote =
      compositionScope === 'inside'
        ? 'internal slice'
        : compositionScope === 'external_dependents'
          ? 'external dependents'
          : 'depends on others'
    if (selectedLayerMeta.showRisks && risks && compositionScope === 'inside') {
      return `${base} · risk: ${risks.summary} · ${scopeNote}`
    }
    return `${base} · ${scopeNote}`
  }, [
    graph,
    risks,
    selectedId,
    selectedLayerMeta?.label,
    selectedLayerMeta?.showRisks,
    selectedLayerMeta?.note,
    showScopeControls,
    compositionScope,
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
            <div className="header__title-left">
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
                        <strong>Imports / Types / Calls / Data</strong> each show one edge family.
                      </li>
                      <li>Single-kind layers hide repeated cable labels.</li>
                    </ul>
                  </section>
                  <section className="help-popout__section">
                    <h3>Slice composition</h3>
                    <ul>
                      <li>
                        <strong>Internal</strong> is wiring among packages owned by this slice.
                      </li>
                      <li>
                        <strong>External dependents</strong> are outside packages that depend on
                        this slice.
                      </li>
                      <li>
                        <strong>Depends on others</strong> are outside packages this slice depends
                        on.
                      </li>
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
                    <h3>Boards</h3>
                    <ul>
                      <li>Switch named boards without restarting the server.</li>
                      <li>
                        Open another window with a different <code>?board=</code> id.
                      </li>
                      <li>
                        Multi-repo boards use <code>--prefix</code> so ids stay unique
                        (for example <code>consilium-chronology</code>).
                      </li>
                      <li>Filter by repo with the Repo control or <code>?repo=</code>.</li>
                      <li>Each board id remembers its own layout in this browser.</li>
                    </ul>
                  </section>
                  <section className="help-popout__section">
                    <h3>Remembered in this browser</h3>
                    <ul>
                      <li>
                        Cable nudges, box positions, and the camera are saved per full board id
                        (prefix included), so two repos never share layout.
                      </li>
                      <li>
                        <strong>Reset view</strong> clears only the layer and focus you are on.
                      </li>
                    </ul>
                  </section>
                </div>
              ) : null}
            </div>
            <p className="header__meta">{summary}</p>
          </div>
          <p className="header__tagline">
            Interactive cable board
            {boardRepo ? ` · ${boardRepo}` : ''}
            {boardLabel ? ` — ${boardLabel}` : ''} for typology{' '}
            <code>assembly-graph.json</code> (imports, roles, wrong-way marks).
          </p>
          <div className="toolbar">
            {boards && repoOptions.length > 0 ? (
              <label className="board-switch">
                <span className="board-switch__label">Repo</span>
                <select
                  className="board-switch__select"
                  value={repoFilter}
                  onChange={(event) => onRepoFilterChange(event.target.value)}
                  aria-label="Cable board repo filter"
                >
                  <option value="">All repos</option>
                  {repoOptions.map((repo) => (
                    <option key={repo} value={repo}>
                      {repo}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {boards && switcherBoards.length > 0 ? (
              <label className="board-switch">
                <span className="board-switch__label">Board</span>
                <select
                  className="board-switch__select"
                  value={boardId ?? ''}
                  onChange={(event) => onBoardChange(event.target.value)}
                  aria-label="Cable board"
                >
                  {switcherGroups.length > 1
                    ? switcherGroups.map((group) => (
                        <optgroup key={group.repo} label={group.label}>
                          {group.boards.map((board) => (
                            <option key={board.id} value={board.id}>
                              {board.label}
                            </option>
                          ))}
                        </optgroup>
                      ))
                    : switcherBoards.map((board) => (
                        <option key={board.id} value={board.id}>
                          {board.label}
                        </option>
                      ))}
                </select>
              </label>
            ) : null}
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
          </div>
          <div className="toolbar toolbar--secondary" ref={legendToolbarRef}>
            {showScopeControls ? (
              <div className="scope-switch" role="tablist" aria-label="Composition scope">
                <span className="scope-switch__label">Scope</span>
                {(
                  [
                    { id: 'inside', label: 'Internal' },
                    { id: 'external_dependents', label: 'External dependents' },
                    { id: 'depends_on_others', label: 'Depends on others' },
                  ] as const
                ).map((scope) => (
                  <button
                    key={scope.id}
                    type="button"
                    className={`layer-switch__button ${
                      compositionScope === scope.id ? 'layer-switch__button--active' : ''
                    }`}
                    aria-pressed={compositionScope === scope.id}
                    title={
                      scope.id === 'inside'
                        ? 'Wiring among packages owned by this slice'
                        : scope.id === 'external_dependents'
                          ? 'Outside packages that depend on this slice'
                          : 'Outside packages this slice depends on'
                    }
                    onClick={() => onScopeClick(scope.id)}
                  >
                    {scope.label}
                  </button>
                ))}
              </div>
            ) : null}
            <div className="toolbar__actions">
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
            </div>
            {legendOpen ? (
              <div
                id="layer-legend-panel"
                className="legend-popout__panel"
                role="dialog"
                aria-label="Wiring legend"
              >
                <h3 className="marks-legend__heading">Layers</h3>
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
                <MarksLegend />
              </div>
            ) : null}
          </div>
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
