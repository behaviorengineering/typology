/**
 * localStorage for assembly-board navigation, cable nudges, and box positions.
 *
 * State is scoped per board: every key is namespaced by the active board id,
 * so two `?board=` windows never share layout, selection, or camera state.
 * Boards served without a registry (legacy `?src=` / `/assembly-graph.json`)
 * use the `default` and `custom` scopes.
 */

export type BoardViewport = { x: number; y: number; zoom: number }
export type NodePos = { x: number; y: number }

export type BoardNavState = {
  layer: string
  selectedId: string | null
  legendOpen: boolean
  viewport: BoardViewport | null
  /** Viewports keyed by nav key for tab returns mid-exploration. */
  viewports: Record<string, BoardViewport>
  cableNudges: Record<string, { bowX: number; bowY: number }>
  /** Node positions keyed by nav key, then node id. */
  nodePositions: Record<string, Record<string, NodePos>>
}

const LEGACY_STORAGE_KEY = 'assembly-board:v1'
const STORAGE_PREFIX = 'assembly-board:v2:'

let activeBoardId = 'default'

/** Switch the persistence scope. Call before any other function in this module. */
export function setActiveBoardId(boardId: string): void {
  const id = boardId.trim()
  activeBoardId = id === '' ? 'default' : id
}

export function getActiveBoardId(): string {
  return activeBoardId
}

function storageKey(boardId: string): string {
  return `${STORAGE_PREFIX}${boardId}`
}

export const defaultBoardState = (): BoardNavState => ({
  layer: 'all',
  selectedId: null,
  legendOpen: false,
  viewport: null,
  viewports: {},
  cableNudges: {},
  nodePositions: {},
})

function normalize(parsed: Partial<BoardNavState>): BoardNavState {
  return {
    ...defaultBoardState(),
    ...parsed,
    viewports: parsed.viewports ?? {},
    cableNudges: parsed.cableNudges ?? {},
    nodePositions: parsed.nodePositions ?? {},
  }
}

/** Nav keys include the board, so saved views never cross boards. */
export function navKey(layer: string, selectedId: string | null): string {
  return `${activeBoardId}::${layer}::${selectedId ?? ''}`
}

export function loadBoardState(): BoardNavState {
  const key = storageKey(activeBoardId)
  try {
    const raw = localStorage.getItem(key)
    if (raw) return normalize(JSON.parse(raw) as Partial<BoardNavState>)
  } catch {
    // Corrupt entry: fall through to legacy adoption, then defaults.
  }
  // First visit for this board: adopt the pre-registry state once so
  // existing users keep node positions. The legacy key is left in place.
  try {
    const legacy = localStorage.getItem(LEGACY_STORAGE_KEY)
    if (legacy) {
      const adopted = normalize(JSON.parse(legacy) as Partial<BoardNavState>)
      try {
        localStorage.setItem(key, JSON.stringify(adopted))
      } catch {
        // quota / private mode: ignore
      }
      return adopted
    }
  } catch {
    // ignore
  }
  return defaultBoardState()
}

export function saveBoardState(patch: Partial<BoardNavState>): BoardNavState {
  const next = { ...loadBoardState(), ...patch }
  try {
    localStorage.setItem(storageKey(activeBoardId), JSON.stringify(next))
  } catch {
    // quota / private mode: ignore
  }
  return next
}

/** Clear saved state for the active board only. */
export function clearBoardState(): void {
  try {
    localStorage.removeItem(storageKey(activeBoardId))
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
