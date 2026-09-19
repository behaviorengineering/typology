import { memo } from 'react'
import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { InboundSocket } from './PlugGlyph'
import { stubMetaLabel } from './scope'

export type PortRef = {
  id: string
  /** Typology role-edge kind when known (imports, fills_dto, …). */
  kind?: string
}

export type PackageNodeData = {
  path: string
  label: string
  inDegree: number
  outDegree: number
  isHub: boolean
  inbound: PortRef[]
  outbound: PortRef[]
  highlighted: boolean
  dimmed: boolean
  inCycle: boolean
  inMesh: boolean
  wrongWay: boolean
  role: string
  modifiers: string[]
  isBoundary?: boolean
  boundaryKind?: string
  ownerId?: string
  /** in = they depend on slice; out = slice depends on them */
  stubDirection?: 'in' | 'out' | 'both'
}

export type PackageFlowNode = Node<PackageNodeData, 'package'>

function boundaryBadge(kind?: string, ownerId?: string): string {
  if (kind === 'library') return ownerId ? `[Library: ${ownerId}]` : '[Library]'
  if (kind === 'slice') return ownerId ? `[Slice: ${ownerId}]` : '[Slice]'
  return '[Unowned]'
}

function PackageNodeView({ data }: NodeProps<PackageFlowNode>) {
  const width = 168 + Math.min(data.outDegree + data.inDegree, 12) * 6
  const className = [
    'pkg-node',
    data.inbound.length > 0 ? 'pkg-node--has-ins' : '',
    data.isBoundary ? 'pkg-node--boundary' : '',
    data.isBoundary && data.stubDirection === 'in' ? 'pkg-node--stub-in' : '',
    data.isBoundary && data.stubDirection === 'out' ? 'pkg-node--stub-out' : '',
    data.isBoundary && data.stubDirection === 'both' ? 'pkg-node--stub-both' : '',
    data.isBoundary && data.boundaryKind === 'library' ? 'pkg-node--boundary-library' : '',
    data.isBoundary && data.boundaryKind === 'unowned' ? 'pkg-node--boundary-unowned' : '',
    !data.isBoundary && data.isHub ? 'pkg-node--hub' : '',
    data.highlighted ? 'pkg-node--hot' : '',
    data.dimmed ? 'pkg-node--dim' : '',
    data.inCycle ? 'pkg-node--cycle' : '',
    data.wrongWay && !data.inCycle ? 'pkg-node--wrong' : '',
    data.inMesh && !data.inCycle && !data.wrongWay ? 'pkg-node--mesh' : '',
  ]
    .filter(Boolean)
    .join(' ')

  const marks = [
    data.inCycle ? 'cycle' : '',
    data.wrongWay ? 'wrong-way' : '',
    data.inMesh ? 'mesh' : '',
  ].filter(Boolean)

  return (
    <div className={className} style={{ minWidth: width }}>
      {data.inbound.map((port, i) => (
        <Handle
          key={`in-${port.id}`}
          id={`in-${port.id}`}
          type="target"
          position={Position.Left}
          className="pkg-handle pkg-handle--in"
          title={port.kind ? `${port.id} (${port.kind})` : port.id}
          style={{ top: `${((i + 1) / (data.inbound.length + 1)) * 100}%` }}
        >
          <InboundSocket kind={port.kind} />
        </Handle>
      ))}
      {data.isBoundary ? (
        <div className="pkg-node__badge">{boundaryBadge(data.boundaryKind, data.ownerId)}</div>
      ) : null}
      <div className="pkg-node__title">{data.label}</div>
      <div className="pkg-node__meta">
        {data.isBoundary
          ? stubMetaLabel(data.stubDirection ?? null)
          : `${data.role ? `${data.role} · ` : ''}in ${data.inDegree} · out ${data.outDegree}${
              marks.length > 0 ? ` · ${marks.join(' · ')}` : ''
            }`}
      </div>
      {data.modifiers.length > 0 ? (
        <div className="pkg-node__modifiers" aria-label="Secondary role modifiers">
          {data.modifiers.map((modifier) => (
            <span
              key={modifier}
              className="pkg-node__modifier-pill"
              title={`Secondary modifier: ${modifier}`}
            >
              +{modifier}
            </span>
          ))}
        </div>
      ) : null}
      <div className="pkg-node__path">{data.path}</div>
      {data.outbound.map((port, i) => (
        <Handle
          key={`out-${port.id}`}
          id={`out-${port.id}`}
          type="source"
          position={Position.Right}
          className="pkg-handle pkg-handle--out"
          title={port.kind ? `${port.id} (${port.kind})` : port.id}
          style={{ top: `${((i + 1) / (data.outbound.length + 1)) * 100}%` }}
        />
      ))}
    </div>
  )
}

export default memo(PackageNodeView)
