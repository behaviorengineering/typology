/** Shared inbound plug glyph: tip always points into the socket (+X). */

export type PlugKind =
  | 'imports'
  | 'fills_dto'
  | 'uses_runner'
  | 'reads_config'
  | 'serves_server'
  | 'composes'
  | 'default'

export function plugKind(kind?: string): PlugKind {
  switch (kind) {
    case 'imports':
    case 'fills_dto':
    case 'uses_runner':
    case 'reads_config':
    case 'serves_server':
    case 'composes':
      return kind
    default:
      return 'default'
  }
}

/** Polygon strings: flat butt on the left, tip at max X (into socket). */
export function plugPoints(kind: PlugKind): string {
  switch (kind) {
    case 'fills_dto':
      return '0.8,2.4 5.2,2.4 5.2,1.6 7.2,1.6 10.4,4 7.2,6.4 5.2,6.4 5.2,5.6 0.8,5.6'
    case 'uses_runner':
      return '0.8,2.5 5.4,2.5 6.2,1.7 10.5,4 6.2,6.3 5.4,5.5 0.8,5.5'
    case 'reads_config':
      return '0.8,2.5 5.6,2.5 7.4,2.1 10.2,4 7.4,5.9 5.6,5.5 0.8,5.5'
    case 'serves_server':
      return '0.8,2.5 4.8,2.5 6.4,1.5 10.6,4 6.4,6.5 4.8,5.5 0.8,5.5'
    case 'composes':
      return '0.8,2.2 5.4,2.2 5.4,1.5 10.4,4 5.4,6.5 5.4,5.8 0.8,5.8'
    case 'imports':
    case 'default':
    default:
      return '0.8,2.5 5.5,2.5 5.5,1.6 10.5,4 5.5,6.4 5.5,5.5 0.8,5.5'
  }
}

export function InboundSocket({
  kind,
  width = 11,
  height = 8,
}: {
  kind?: string
  width?: number
  height?: number
}) {
  const k = plugKind(kind)
  return (
    <svg
      className={`pkg-socket pkg-socket--${k}`}
      viewBox="0 0 12 8"
      width={width}
      height={height}
      aria-hidden
    >
      <path
        d="M7 1 h4 a1 1 0 0 1 1 1 v4 a1 1 0 0 1 -1 1 H7"
        fill="#f8fafc"
        stroke="#64748b"
        strokeWidth="0.85"
        strokeLinejoin="round"
      />
      <polygon
        className="pkg-socket__plug"
        points={plugPoints(k)}
        fill="#0369a1"
        stroke="#0f172a"
        strokeWidth="0.75"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export function OutboundJack({ size = 6 }: { size?: number }) {
  return (
    <span
      className="pkg-jack"
      style={{ width: size, height: size }}
      aria-hidden
    />
  )
}
