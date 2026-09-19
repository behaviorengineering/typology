import type { ReactNode } from 'react'
import { InboundSocket, OutboundJack } from './PlugGlyph'

type MarkEntry = {
  id: string
  title: string
  read: string
  sample: ReactNode
}

function MiniPkg({
  label,
  variant = 'plain',
}: {
  label: string
  variant?: 'plain' | 'boundary' | 'wrong' | 'mesh' | 'cycle' | 'hub'
}) {
  return (
    <span className={`mark-sample__pkg mark-sample__pkg--${variant}`} aria-hidden>
      {label}
    </span>
  )
}

function CableSample({
  stroke,
  dash,
  label,
}: {
  stroke: string
  dash?: boolean
  label?: string
}) {
  return (
    <span className="mark-sample__cable-wrap" aria-hidden>
      <span
        className={`mark-sample__cable ${dash ? 'mark-sample__cable--dash' : ''}`}
        style={{ borderTopColor: stroke }}
      />
      {label ? <span className="mark-sample__cable-label">{label}</span> : null}
    </span>
  )
}

function WiringSample({ kind }: { kind: string }) {
  return (
    <span className="mark-sample__wiring" aria-hidden>
      <MiniPkg label="pkg" />
      <OutboundJack size={5} />
      <CableSample stroke="#64748b" />
      <InboundSocket kind={kind} width={14} height={10} />
      <MiniPkg label="pkg" />
    </span>
  )
}

const architecturalLayers = [
  {
    id: 'layer-0',
    tag: '0',
    title: 'Entrypoint',
    read: 'CLI commands and main bootstraps that start the application.',
  },
  {
    id: 'layer-1',
    tag: '1',
    title: 'Boot & Delivery',
    read: 'Servers, containers, and request delivery surfaces.',
  },
  {
    id: 'layer-2',
    tag: '2',
    title: 'Domain & Adapters',
    read: 'Domain logic, workers, queues, pipelines, and external adapters.',
  },
  {
    id: 'layer-3',
    tag: '3',
    title: 'Data Transfer',
    read: 'DTOs, contracts, and request or response payload shapes.',
  },
  {
    id: 'layer-4',
    tag: '4',
    title: 'Infrastructure & Config',
    read: 'Config, observability, path roots, and other support packages.',
  },
] as const

/** Visual + cold-read entries for the Legend panel. */
export const legendMarks: MarkEntry[] = [
  {
    id: 'import',
    title: 'Ordinary cable',
    read: 'One package imports another. Solid gray line. Plug shape changes with the cable kind.',
    sample: <WiringSample kind="imports" />,
  },
  {
    id: 'wrong-way',
    title: 'Wrong-way cable',
    read: 'An inner package reaches outward (for example a dto importing domain core). Purple line; motion can make it look dashed.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="dto" variant="wrong" />
        <OutboundJack size={5} />
        <CableSample stroke="#c026d3" label="imports" />
        <InboundSocket kind="imports" width={14} height={10} />
        <MiniPkg label="core" variant="wrong" />
      </span>
    ),
  },
  {
    id: 'missing-binding',
    title: 'Missing binding',
    read: 'The cable crosses a slice edge with no matching sliceBinding in the catalog. Amber dashed line.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="owned" />
        <OutboundJack size={5} />
        <CableSample stroke="#d97706" dash label="missing" />
        <InboundSocket kind="imports" width={14} height={10} />
        <MiniPkg label="out" variant="boundary" />
      </span>
    ),
  },
  {
    id: 'cycle',
    title: 'Cycle',
    read: 'Packages import each other in a loop. Red border and red cable on Imports.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="a" variant="cycle" />
        <CableSample stroke="#b91c1c" />
        <MiniPkg label="b" variant="cycle" />
      </span>
    ),
  },
  {
    id: 'mesh',
    title: 'Mesh',
    read: 'A dense pocket of mutual wiring in the middle of the graph. Amber package border on Imports.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="hub" variant="mesh" />
        <CableSample stroke="#64748b" />
        <MiniPkg label="hub" variant="mesh" />
      </span>
    ),
  },
  {
    id: 'boundary',
    title: 'Boundary stub',
    read: 'A package outside the selected slice, kept so cross-slice cables stay visible. Dashed gray card.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="stub" variant="boundary" />
      </span>
    ),
  },
  {
    id: 'hub',
    title: 'Hub package',
    read: 'Many cables in this view (fan-in or fan-out). Thicker dark border.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden>
        <MiniPkg label="hub" variant="hub" />
      </span>
    ),
  },
  {
    id: 'modifier',
    title: 'Role modifier',
    read: 'Secondary capability detected alongside primary role (e.g. +config, +server, +container). Shown as pill badges.',
    sample: (
      <span className="mark-sample__wiring" aria-hidden style={{ gap: '4px' }}>
        <span className="pkg-node__modifier-pill">+config</span>
        <span className="pkg-node__modifier-pill">+locator</span>
      </span>
    ),
  },
]

export function MarksLegend() {
  return (
    <div className="marks-legend" aria-label="Board marks">
      <div className="marks-legend__section">
        <h3 className="marks-legend__heading">Architectural layers</h3>
        <ul className="marks-legend__layer-list">
          {architecturalLayers.map((layer) => (
            <li key={layer.id} className="marks-legend__layer-row">
              <span className="marks-legend__layer-pill">{layer.tag}</span>
              <span className="marks-legend__copy">
                <span className="marks-legend__title">{layer.title}</span>
                <span className="marks-legend__read">{layer.read}</span>
              </span>
            </li>
          ))}
        </ul>
        <p className="marks-legend__note">
          Outer layers may depend inward. If an inner layer imports outward, Typology marks a
          wrong-way cable.
        </p>
      </div>
      <h3 className="marks-legend__heading">Marks</h3>
      <ul className="marks-legend__list">
        {legendMarks.map((mark) => (
          <li key={mark.id} className="marks-legend__row">
            <div className="marks-legend__sample">{mark.sample}</div>
            <div className="marks-legend__copy">
              <span className="marks-legend__title">{mark.title}</span>
              <span className="marks-legend__read">{mark.read}</span>
            </div>
          </li>
        ))}
      </ul>
      <p className="marks-legend__hint">
        Cycle, mesh, and wrong-way marks show on Imports. Missing binding shows on boundary
        cables (External dependents / Depends on others).
      </p>
    </div>
  )
}
