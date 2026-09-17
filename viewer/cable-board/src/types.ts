export type GraphNode = {
  id: string
  path: string
  inDegree: number
  outDegree: number
  imports: string[]
  importedBy: string[]
  isHub: boolean
  isLeaf: boolean
  role?: string
  roleConfidence?: number
  layer?: number
  isBoundary?: boolean
  boundaryKind?: 'slice' | 'library' | 'unowned' | string
  ownerId?: string
}

export type GraphEdge = {
  id: string
  source: string
  target: string
  roleKind?: string
  wrongWay?: boolean
  wrongWayReason?: string
  bindingStatus?: 'declared' | 'missing' | string
  boundaryKind?: string
}

export type PackageGraph = {
  nodes: GraphNode[]
  edges: GraphEdge[]
  slice?: string
}

export function pathToId(path: string): string {
  const s = path.replace(/^\.\//, '').replace(/\/$/, '')
  if (!s || s === '.') return 'root'
  return s.replaceAll('/', '__')
}

export function shortLabel(path: string): string {
  const s = path.replace(/^\.\//, '')
  if (s.startsWith('cmd/')) return s
  if (s.startsWith('internal/')) return s.slice('internal/'.length)
  return s
}
