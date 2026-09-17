/** localStorage for assembly-board navigation, cable nudges, and box positions. */

export type BoardViewport = { x: number; y: number; zoom: number }
export type NodePos = { x: number; y: number }

export type BoardNavState = {
  layer: string
  selectedId: string | null
  legendOpen: boolean
  viewport: BoardViewport | null
  /** Viewports keyed by `${layer}::${selectedId ?? ''}` for tab returns mid-exploration. */
  viewports: Record<string, BoardViewport>
  cableNudges: Record<string, { bowX: number; bowY: number }>
  /** Node positions keyed by nav key, then node id. */
  nodePositions: Record<string, Record<string, NodePos>>
}

const STORAGE_KEY = 'assembly-board:v1'

export const defaultBoardState = (): BoardNavState => ({
  layer: 'all',
  selectedId: null,
  legendOpen: false,
  viewport: null,
  viewports: {},
  cableNudges: {},
  nodePositions: {},
})

export function navKey(layer: string, selectedId: string | null): string {
  return `${layer}::${selectedId ?? ''}`
}

export function loadBoardState(): BoardNavState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaultBoardState()
    const parsed = JSON.parse(raw) as Partial<BoardNavState>
    return {
      ...defaultBoardState(),
      ...parsed,
      viewports: parsed.viewports ?? {},
      cableNudges: parsed.cableNudges ?? {},
      nodePositions: parsed.nodePositions ?? {},
    }
  } catch {
    return defaultBoardState()
  }
}

export function saveBoardState(patch: Partial<BoardNavState>): BoardNavState {
  const next = { ...loadBoardState(), ...patch }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    // quota / private mode: ignore
  }
  return next
}

export function clearBoardState(): void {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}

/** Clear saved camera, box positions, and cable nudges for one nav view only. */
export function resetViewState(
  key: string,
  edgeIds: string[],
  nudges: Map<string, { bowX: number; bowY: number }>,
): BoardNavState {
  const state = loadBoardState()
  const { [key]: _dropVp, ...viewports } = state.viewports
  const { [key]: _dropPos, ...nodePositions } = state.nodePositions
  for (const id of edgeIds) {
    nudges.delete(id)
  }
  const cableNudges: Record<string, { bowX: number; bowY: number }> = {}
  for (const [id, value] of nudges) {
    cableNudges[id] = value
  }
  return saveBoardState({
    viewports,
    nodePositions,
    cableNudges,
    viewport: null,
  })
}

export function saveCableNudges(nudges: Map<string, { bowX: number; bowY: number }>): void {
  const cableNudges: Record<string, { bowX: number; bowY: number }> = {}
  for (const [id, value] of nudges) {
    cableNudges[id] = value
  }
  saveBoardState({ cableNudges })
}

export function hydrateCableNudges(
  target: Map<string, { bowX: number; bowY: number }>,
): void {
  const { cableNudges } = loadBoardState()
  target.clear()
  for (const [id, value] of Object.entries(cableNudges)) {
    if (value && typeof value.bowX === 'number' && typeof value.bowY === 'number') {
      target.set(id, value)
    }
  }
}

export function applySavedNodePositions<T extends { id: string; position: NodePos }>(
  nodes: T[],
  key: string,
): T[] {
  const saved = loadBoardState().nodePositions[key]
  if (!saved) return nodes
  return nodes.map((node) => {
    const pos = saved[node.id]
    return pos ? { ...node, position: { x: pos.x, y: pos.y } } : node
  })
}

export function saveNodePositionsForKey(
  key: string,
  nodes: Array<{ id: string; position: NodePos }>,
): void {
  const state = loadBoardState()
  const nextForKey: Record<string, NodePos> = { ...(state.nodePositions[key] ?? {}) }
  for (const node of nodes) {
    nextForKey[node.id] = { x: node.position.x, y: node.position.y }
  }
  saveBoardState({
    nodePositions: {
      ...state.nodePositions,
      [key]: nextForKey,
    },
  })
}
