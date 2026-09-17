import {
  BaseEdge,
  EdgeLabelRenderer,
  useReactFlow,
  type Edge,
  type EdgeProps,
} from '@xyflow/react'
import { useCallback, useRef, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react'
import { hydrateCableNudges, saveCableNudges } from './boardPersist'

export type WiringEdgeData = {
  /** Sideways bow in px (shifts both control points). */
  bowX?: number
  /** Vertical bow in px (positive = down / toward +Y). */
  bowY?: number
  /** Layout-computed bows; double-click restores these. */
  autoBowX?: number
  autoBowY?: number
  /** Handle extension along X as fraction of span. */
  curvature?: number
}

export type WiringFlowEdge = Edge<WiringEdgeData, 'wiring'>

/** Cable midpoint overrides; hydrated from localStorage. */
export const cableNudges = new Map<string, { bowX: number; bowY: number }>()
hydrateCableNudges(cableNudges)

export function applyCableNudges(edges: Edge[]): Edge[] {
  return edges.map((edge) => {
    const prev =
      edge.data && typeof edge.data === 'object'
        ? (edge.data as WiringEdgeData)
        : ({} as WiringEdgeData)
    const nudge = cableNudges.get(edge.id)
    const autoBowX = prev.autoBowX ?? 0
    const autoBowY = prev.autoBowY ?? prev.bowY ?? 0
    return {
      ...edge,
      type: 'wiring' as const,
      data: {
        autoBowX,
        autoBowY,
        bowX: nudge?.bowX ?? autoBowX,
        bowY: nudge?.bowY ?? autoBowY,
        curvature: prev.curvature,
      },
      labelStyle: undefined,
      labelBgStyle: undefined,
      labelBgPadding: undefined,
      labelBgBorderRadius: undefined,
    }
  })
}

function cubicPath(
  p0: { x: number; y: number },
  p1: { x: number; y: number },
  p2: { x: number; y: number },
  p3: { x: number; y: number },
) {
  return `M ${p0.x} ${p0.y} C ${p1.x} ${p1.y} ${p2.x} ${p2.y} ${p3.x} ${p3.y}`
}

function pointOnCubic(
  t: number,
  p0: { x: number; y: number },
  p1: { x: number; y: number },
  p2: { x: number; y: number },
  p3: { x: number; y: number },
) {
  const u = 1 - t
  const tt = t * t
  const uu = u * u
  return {
    x: u * uu * p0.x + 3 * uu * t * p1.x + 3 * u * tt * p2.x + tt * t * p3.x,
    y: u * uu * p0.y + 3 * uu * t * p1.y + 3 * u * tt * p2.y + tt * t * p3.y,
  }
}

/** Midpoint of our control scheme sits at chord mid + 0.75 * bow. */
const BOW_MID_FACTOR = 0.75

export function wiringControlPoints(
  sourceX: number,
  sourceY: number,
  targetX: number,
  targetY: number,
  bowX: number,
  bowY: number,
  curvature = 0.35,
) {
  const span = Math.abs(targetX - sourceX)
  const dx = Math.max(span * curvature, 48)
  const p0 = { x: sourceX, y: sourceY }
  const p3 = { x: targetX, y: targetY }
  const p1 = { x: sourceX + dx + bowX, y: sourceY + bowY }
  const p2 = { x: targetX - dx + bowX, y: targetY + bowY }
  return { p0, p1, p2, p3 }
}

/**
 * Bezier cable: drag the wire or its label to pull the midpoint (reshapes the cable).
 * Double-click restores the layout bow.
 */
export default function WiringEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  style,
  markerEnd,
  label,
  data,
}: EdgeProps<WiringFlowEdge>) {
  const { setEdges, getZoom } = useReactFlow()
  const dragRef = useRef<{
    pointerId: number
    startClientX: number
    startClientY: number
    originBowX: number
    originBowY: number
  } | null>(null)

  const bowX = data?.bowX ?? 0
  const bowY = data?.bowY ?? 0
  const autoBowX = data?.autoBowX ?? 0
  const autoBowY = data?.autoBowY ?? 0
  const curvature = data?.curvature ?? (bowX === 0 && bowY === 0 ? 0.25 : 0.35)

  const { p0, p1, p2, p3 } = wiringControlPoints(
    sourceX,
    sourceY,
    targetX,
    targetY,
    bowX,
    bowY,
    curvature,
  )
  const path = cubicPath(p0, p1, p2, p3)
  const labelPt = pointOnCubic(0.5, p0, p1, p2, p3)

  const writeBow = useCallback(
    (nextBowX: number, nextBowY: number, persistNudge = true) => {
      if (persistNudge) {
        cableNudges.set(id, { bowX: nextBowX, bowY: nextBowY })
        saveCableNudges(cableNudges)
      }
      setEdges((edges) =>
        edges.map((edge) => {
          if (edge.id !== id) return edge
          const prev =
            edge.data && typeof edge.data === 'object'
              ? (edge.data as WiringEdgeData)
              : ({} as WiringEdgeData)
          return {
            ...edge,
            data: {
              ...prev,
              bowX: nextBowX,
              bowY: nextBowY,
              autoBowX: prev.autoBowX ?? autoBowX,
              autoBowY: prev.autoBowY ?? autoBowY,
            },
          }
        }),
      )
    },
    [id, setEdges, autoBowX, autoBowY],
  )

  const beginDrag = (
    event: ReactPointerEvent,
    captureEl: Element & { setPointerCapture(id: number): void },
  ) => {
    if (event.button !== 0) return
    event.preventDefault()
    event.stopPropagation()
    captureEl.setPointerCapture(event.pointerId)
    dragRef.current = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      originBowX: bowX,
      originBowY: bowY,
    }
  }

  const onPointerMove = (event: ReactPointerEvent) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return
    event.preventDefault()
    event.stopPropagation()
    const zoom = getZoom() || 1
    const dFlowX = (event.clientX - drag.startClientX) / zoom
    const dFlowY = (event.clientY - drag.startClientY) / zoom
    // Curve midpoint ≈ chordMid + 0.75*bow; move that midpoint with the pointer.
    writeBow(
      drag.originBowX + dFlowX / BOW_MID_FACTOR,
      drag.originBowY + dFlowY / BOW_MID_FACTOR,
    )
  }

  const onPointerUp = (event: ReactPointerEvent) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return
    dragRef.current = null
    const target = event.currentTarget as Element & { releasePointerCapture(id: number): void }
    try {
      target.releasePointerCapture(event.pointerId)
    } catch {
      // already released
    }
  }

  const onDoubleClick = (event: { preventDefault(): void; stopPropagation(): void }) => {
    event.preventDefault()
    event.stopPropagation()
    cableNudges.delete(id)
    saveCableNudges(cableNudges)
    writeBow(autoBowX, autoBowY, false)
  }

  const labelBoxStyle: CSSProperties = {
    position: 'absolute',
    transform: `translate(-50%, -50%) translate(${labelPt.x}px, ${labelPt.y}px)`,
    pointerEvents: 'all',
    zIndex: 12,
  }

  return (
    <>
      {/* Wide invisible hit target so the cable itself is grabable. */}
      <path
        d={path}
        fill="none"
        stroke="transparent"
        strokeWidth={22}
        className="nodrag nopan"
        style={{ cursor: 'grab', pointerEvents: 'stroke' }}
        onPointerDown={(e) => beginDrag(e, e.currentTarget)}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onDoubleClick={onDoubleClick}
      />
      <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />
      {label ? (
        <EdgeLabelRenderer>
          <div
            className="wiring-edge-label nodrag nopan"
            style={labelBoxStyle}
            title="Drag label or cable to reshape · double-click to reset"
            onPointerDown={(e) => beginDrag(e, e.currentTarget)}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={onPointerUp}
            onDoubleClick={onDoubleClick}
          >
            {String(label)}
          </div>
        </EdgeLabelRenderer>
      ) : null}
    </>
  )
}
