/** Named cable board registry (viewer/cable-board/public/boards.json). */

export type BoardEntry = {
  id: string
  label: string
  graph: string
  description?: string
}

export type BoardsManifest = {
  defaultBoard?: string
  boards: BoardEntry[]
}

export const BOARDS_MANIFEST_PATH = '/boards.json'
export const LEGACY_BOARD_PATH = '/assembly-graph.json'

/**
 * Stable board ids: lowercase, digits, hyphens. An id appears in `?board=`
 * URLs, storage keys, and `public/boards/<id>/` paths, so it must stay
 * URL-safe and filesystem-safe.
 */
export function isValidBoardId(id: string): boolean {
  return /^[a-z0-9][a-z0-9-]*$/.test(id)
}

export function parseManifest(raw: unknown): BoardsManifest | null {
  if (!raw || typeof raw !== 'object') return null
  const doc = raw as { defaultBoard?: unknown; boards?: unknown }
  if (!Array.isArray(doc.boards)) return null
  const boards: BoardEntry[] = []
  for (const entry of doc.boards) {
    if (!entry || typeof entry !== 'object') return null
    const candidate = entry as Record<string, unknown>
    if (typeof candidate.id !== 'string' || !isValidBoardId(candidate.id)) return null
    if (typeof candidate.label !== 'string' || !candidate.label.trim()) return null
    if (typeof candidate.graph !== 'string' || !candidate.graph.trim()) return null
    const parsed: BoardEntry = {
      id: candidate.id,
      label: candidate.label,
      graph: candidate.graph,
    }
    if (typeof candidate.description === 'string' && candidate.description.trim()) {
      parsed.description = candidate.description
    }
    boards.push(parsed)
  }
  if (boards.length === 0) return null
  const manifest: BoardsManifest = { boards }
  if (
    typeof doc.defaultBoard === 'string' &&
    boards.some((board) => board.id === doc.defaultBoard)
  ) {
    manifest.defaultBoard = doc.defaultBoard
  }
  return manifest
}

/** Returns null when no registry is deployed (legacy single-file board). */
export async function fetchBoardsManifest(): Promise<BoardsManifest | null> {
  let res: Response
  try {
    res = await fetch(BOARDS_MANIFEST_PATH)
  } catch {
    return null
  }
  if (!res.ok) return null
  try {
    return parseManifest((await res.json()) as unknown)
  } catch {
    return null
  }
}

/** Manifest-first default: explicit defaultBoard, else the first entry. */
export function defaultBoardId(manifest: BoardsManifest): string {
  if (manifest.defaultBoard) return manifest.defaultBoard
  return manifest.boards[0].id
}
