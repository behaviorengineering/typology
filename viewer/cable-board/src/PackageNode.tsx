import { memo, useState } from 'react'
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
  roleConfidence?: number
  modifiers: string[]
  layer?: number
  doc?: string
  evidence?: string[]
  exportedDecls?: string[]
  exportedFuncs?: string[]
  isBoundary?: boolean
  boundaryKind?: string
  ownerId?: string
  /** in = they depend on slice; out = slice depends on them */
  stubDirection?: 'in' | 'out' | 'both'
}

export type PackageFlowNode = Node<PackageNodeData, 'package'>

const ROLE_DESCRIPTIONS: Record<string, string> = {
  container: 'Dependency injection hub & service registry managing runtime lifecycle and component wiring.',
  entrypoint: 'Application entrypoint with main() bootstrapping the command or service.',
  server: 'HTTP/gRPC delivery surface registering routes, middleware, and handlers.',
  dto: 'Data transfer objects and serialization types with JSON schema representations.',
  worker: 'Background job execution implementing Kind/Validate/Run task handlers.',
  queue: 'Job queue management and task dispatch infrastructure.',
  config: 'Configuration loading, environment validation, and option structs.',
  locator: 'Path and resource resolution for repositories, instances, and static assets.',
  crypto: 'Cryptographic operations, key management, and age encryption primitives.',
  prompt: 'DSPy module signatures, prompt templates, and structured LLM inputs.',
  adapter: 'External service clients and foreign integration wrappers.',
  pipeline: 'Sequential execution stages, job runners, and orchestration workflows.',
  view: 'UI layout, component rendering, and presentation logic.',
  export: 'Document generation, artifact builders, and serialization export.',
  validation: 'Input validation rules, schema conformance, and issue reporting.',
  observability: 'OpenTelemetry, Prometheus metrics, tracing, and diagnostics.',
  exec_runner: 'OS command execution and sub-process supervision.',
  aggregator: 'Internal service coordinator aggregating multiple domain packages.',
  unknown: 'Internal package without strong architectural role indicators.',
}

const MODIFIER_DESCRIPTIONS: Record<string, string> = {
  config: 'Configuration schema & loader',
  server: 'HTTP / API endpoints',
  worker: 'Background job handler',
  queue: 'Task enqueue & queue operations',
  cli: 'CLI command dispatch & flags',
  locator: 'Path locator & directory root lookup',
  crypto: 'Cryptographic encryption & keys',
  prompt: 'DSPy / prompt instruction surface',
  container: 'Runtime container & lifecycle hooks',
  validation: 'Validation rules & reports',
  adapter: 'External client / network adapter',
  pipeline: 'Pipeline runner & step registry',
  view: 'View models & UI templates',
  export: 'Data export & artifact generation',
  observability: 'Metrics, tracing & monitoring',
  exec_runner: 'Process execution & runner',
}

function layerLabel(layer?: number): string {
  switch (layer) {
    case 0:
      return 'Layer 0 · Entrypoint'
    case 1:
      return 'Layer 1 · Boot & Delivery'
    case 2:
      return 'Layer 2 · Domain & Adapters'
    case 3:
      return 'Layer 3 · Data Transfer (DTO)'
    case 4:
      return 'Layer 4 · Infrastructure & Config'
    default:
      return layer !== undefined ? `Layer ${layer}` : 'Unlayered'
  }
}

function PackageRolloverPanel({ data }: { data: PackageNodeData }) {
  const roleDesc = ROLE_DESCRIPTIONS[data.role] || ROLE_DESCRIPTIONS.unknown
  const confidencePercent =
    typeof data.roleConfidence === 'number' ? Math.round(data.roleConfidence * 100) : null

  return (
    <div
      className="pkg-rollover"
      role="tooltip"
      aria-label={`Details for ${data.label}`}
      onClick={(e) => e.stopPropagation()}
    >
      <div className="pkg-rollover__header">
        <div className="pkg-rollover__eyebrow">{layerLabel(data.layer)}</div>
        <div className="pkg-rollover__title-row">
          <span className="pkg-rollover__name">{data.label}</span>
          <div className="pkg-rollover__role-badge">
            <span className="pkg-rollover__role">{data.role || 'unknown'}</span>
            {confidencePercent !== null ? (
              <span className="pkg-rollover__conf">{confidencePercent}%</span>
            ) : null}
          </div>
        </div>
        <div className="pkg-rollover__path">{data.path}</div>
      </div>

      <div className="pkg-rollover__section">
        <div className="pkg-rollover__doc">
          {data.doc ? data.doc : <span className="pkg-rollover__fallback">{roleDesc}</span>}
        </div>
      </div>

      {data.modifiers.length > 0 ? (
        <div className="pkg-rollover__section">
          <div className="pkg-rollover__subhead">Secondary Modifiers</div>
          <div className="pkg-rollover__modifiers-list">
            {data.modifiers.map((mod) => (
              <div key={mod} className="pkg-rollover__modifier-item">
                <span className="pkg-node__modifier-pill">+{mod}</span>
                <span className="pkg-rollover__modifier-desc">
                  {MODIFIER_DESCRIPTIONS[mod] || 'Blended capability'}
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {data.evidence && data.evidence.length > 0 ? (
        <div className="pkg-rollover__section">
          <div className="pkg-rollover__subhead">AST Evidence</div>
          <div className="pkg-rollover__chips">
            {data.evidence.map((ev) => (
              <span key={ev} className="pkg-rollover__chip">
                {ev}
              </span>
            ))}
          </div>
        </div>
      ) : null}

      {(data.exportedDecls && data.exportedDecls.length > 0) ||
      (data.exportedFuncs && data.exportedFuncs.length > 0) ? (
        <div className="pkg-rollover__section">
          <div className="pkg-rollover__subhead">Exported Surface</div>
          {data.exportedDecls && data.exportedDecls.length > 0 ? (
            <div className="pkg-rollover__surface-row">
              <span className="pkg-rollover__surface-label">Types:</span>
              <div className="pkg-rollover__chips">
                {data.exportedDecls.map((d) => (
                  <code key={d} className="pkg-rollover__code-chip">
                    {d}
                  </code>
                ))}
              </div>
            </div>
          ) : null}
          {data.exportedFuncs && data.exportedFuncs.length > 0 ? (
            <div className="pkg-rollover__surface-row">
              <span className="pkg-rollover__surface-label">Funcs:</span>
              <div className="pkg-rollover__chips">
                {data.exportedFuncs.map((f) => (
                  <code key={f} className="pkg-rollover__code-chip">
                    {f}()
                  </code>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      <div className="pkg-rollover__footer">
        <div className="pkg-rollover__stats">
          <span>
            Inbound: <strong>{data.inDegree}</strong>
          </span>
          <span className="pkg-rollover__dot">·</span>
          <span>
            Outbound: <strong>{data.outDegree}</strong>
          </span>
        </div>
        {data.wrongWay ? (
          <div className="pkg-rollover__alert pkg-rollover__alert--wrong">
            Wrong-way cable: inner layer imports outer layer
          </div>
        ) : null}
        {data.inCycle ? (
          <div className="pkg-rollover__alert pkg-rollover__alert--cycle">
            Circular dependency: imports peer in a loop
          </div>
        ) : null}
        {data.inMesh ? (
          <div className="pkg-rollover__alert pkg-rollover__alert--mesh">
            Mesh coupling: high mutual wiring cluster
          </div>
        ) : null}
      </div>
    </div>
  )
}

function boundaryBadge(kind?: string, ownerId?: string): string {
  if (kind === 'library') return ownerId ? `[Library: ${ownerId}]` : '[Library]'
  if (kind === 'slice') return ownerId ? `[Slice: ${ownerId}]` : '[Slice]'
  return '[Unowned]'
}

function PackageNodeView({ data }: NodeProps<PackageFlowNode>) {
  const [hovered, setHovered] = useState(false)
  const showPanel = (hovered || data.highlighted) && !data.dimmed
  const width = 168 + Math.min(data.outDegree + data.inDegree, 12) * 6
  const className = [
    'pkg-node',
    showPanel ? 'pkg-node--active-popover' : '',
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
    <div
      className={className}
      style={{ minWidth: width, zIndex: showPanel ? 60 : undefined }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {showPanel ? <PackageRolloverPanel data={data} /> : null}
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
