---
name: typology-catalog
description: >-
  Author Typology catalogs (YAML or catalog.Typology in Go): slices, components,
  surfaces, subprograms, actuators, opRuns, sliceBindings, componentBindings. Load when
  writing architecture/typology.yaml, calling catalog.LoadYAML or SaveYAML, or
  mapping a product contract onto the Typology model.
---

# Typology catalog

**Moral:** A slice is a bounded context. A component is a package. A surface is a built interaction artefact (UI, CLI, or API) that owns packages. A subprogram is a standing program. An opRun is one gated invocation. An actuator is a signal-triggered emit. Do not collapse those five.

**Types:** `catalog/types.go` · **Fixture:** `testdata/tiny-module/architecture/typology.yaml` · **I/O:** `catalog.LoadYAML`, `catalog.SaveYAML`, `catalog.ValidateStructure`

## When to load

- Adding or editing `architecture/typology.yaml` (or `typology.yaml` at repo root)
- Building a `catalog.Typology` in Go and writing it with `SaveYAML`
- Naming subprograms, actuators, opRuns, or bindings
- A validate error about `runs`, `actuates`, `ownerComponent`, `signals`, or `emits`

## Model

| Type | Role |
|------|------|
| `Typology` | Whole map (`id`, `slices`, optional bindings) |
| `Slice` | Bounded context (`owns`, `surfaces`, `opRuns`, `subprograms`, `actuators`, `docs`) |
| `Component` | Package path. Domain packages live on `owns[]`. Interaction packages live under a `Surface`. |
| `Surface` | Built interaction artefact (`kind`: `ui`, `cli`, or `api`) with nested `components[]` (id + path only). |
| `OpRun` | One gated invocation (CLI, HTTP, human, signal, or later schedule). Optional `runs` or `actuates`, not both. |
| `Subprogram` | Standing program: `input`, `output`, optional `store`, `gate`. Origin for `store` paths when first written. |
| `Actuator` | Signal in, emit out (usually past the instance edge). Requires `signals` and `emits`. |
| `SliceBinding` | Slice to slice: `consumes` or `reads` |
| `ComponentBinding` | Component to component: `must`, `must_not`, or `reads` |

Gates: `auto` | `test` | `human`.

Default catalog path: `architecture/typology.yaml`. Default docs root: `docs/develop`.

## Core constraints

**CONSTRAINT:** Subprogram, actuator, and opRun MUST set `ownerComponent` to a component id on the same slice (`owns[]` or a surface's `components[]`).

- MUST: `ownerComponent` matches a component from `Slice.AllComponents()` on that slice
- MUST NOT: invent an owner that is not in `owns` or `surfaces`

Enforcement: `catalog.ValidateStructure` (`ownerComponent … not in slice owns`)
Violation: STOP, add the component or fix the id, re-validate

CORRECT:
```yaml
owns:
  - id: billing-store
    path: internal/billing/store
    layer: domain
subprograms:
  - id: invoice
    ownerComponent: billing-store
    input: invoice request
    output: invoice record
    gate: auto
```

PROHIBITED:
```yaml
subprograms:
  - id: invoice
    ownerComponent: billing-engine   # not in owns
```

**CONSTRAINT:** OpRun `runs` and `actuates` are mutually exclusive. `runs` MUST name a subprogram on this slice. `actuates` MUST name an actuator on this slice. Empty both is allowed (invocation with no named program).

- MUST: at most one of `runs` or `actuates`
- MUST NOT: set both; MUST NOT `runs` a missing subprogram; MUST NOT `actuates` a missing actuator

Enforcement: `ValidateStructure` (`runs and actuates are mutually exclusive`; `runs … is not a subprogram`; `actuates … is not an actuator`)
Violation: STOP, split into two opRuns or add the missing program, re-validate

CORRECT:
```yaml
opRuns:
  - id: mint-invoice
    ownerComponent: billing-store
    gate: auto
    runs: invoice
  - id: push-invoice
    ownerComponent: billing-http
    gate: auto
    actuates: invoice-webhook
```

PROHIBITED:
```yaml
opRuns:
  - id: mixed
    ownerComponent: billing-store
    runs: invoice
    actuates: invoice-webhook
```

**CONSTRAINT:** Actuator id MUST NOT collide with a subprogram id on the same slice. Actuator MUST set non-empty `signals` and `emits`.

- MUST: unique id vs subprograms; at least one signal; at least one emit
- MUST NOT: reuse a subprogram id; MUST NOT leave signals or emits empty

Enforcement: `ValidateStructure` (`id already used by a subprogram`; `missing signals`; `missing emits`)
Violation: STOP, rename or fill fields, re-validate

CORRECT:
```yaml
actuators:
  - id: invoice-webhook
    ownerComponent: billing-http
    signals: [invoice.minted]
    emits: [webhook]
    gate: auto
```

PROHIBITED:
```yaml
subprograms:
  - id: notify
actuators:
  - id: notify          # collision
    signals: []
    emits: []
```

**CONSTRAINT:** Cross-slice coupling MUST be a `SliceBinding`. A `ComponentBinding` that crosses slices MUST have a matching `SliceBinding` (either direction).

- MUST: `kind` is `consumes` or `reads` on slice bindings
- MUST: `rule` is `must`, `must_not`, or `reads` on component bindings
- MUST NOT: imply consume of another slice's store only in prose; MUST NOT cross-slice component binding with no slice binding

Enforcement: `ValidateStructure` (`ComponentBinding … crosses slices without SliceBinding`; unknown from/to ids)
Violation: STOP, add `sliceBindings` (and the component binding if needed), re-validate

CORRECT:
```yaml
sliceBindings:
  - from: billing
    to: ledger
    kind: reads
componentBindings:
  - from: billing-store
    to: ledger-core
    rule: reads
```

PROHIBITED:
```yaml
# billing-store imports ledger-core, no sliceBindings row
componentBindings:
  - from: billing-store
    to: ledger-core
    rule: reads
```

**CONSTRAINT:** Slice `owns[]` is domain-only. Interaction packages MUST live under `surfaces[]`.

- MUST: `owns[].layer` is `domain`
- MUST: each surface sets `kind` (`ui`, `cli`, or `api`) and lists nested components (id + path)
- MUST NOT: put `layer: interaction` on `owns[]`; MUST NOT put `layer` or `kind` on nested surface components

Enforcement: `ValidateStructure` (`interaction packages belong on surfaces, not owns`; `surface … missing kind`)
Violation: STOP, move packages onto `surfaces[]`, re-validate

CORRECT:
```yaml
owns:
  - id: billing-store
    path: internal/billing/store
    layer: domain
surfaces:
  - id: billing-api
    kind: api
    components:
      - id: billing-http
        path: internal/billing/httpapi
```

PROHIBITED:
```yaml
owns:
  - id: billing-http
    path: internal/billing/httpapi
    layer: interaction
    kind: api
```

**CONSTRAINT:** A component is a package. MUST NOT treat a component or surface as a subprogram or actuator.

- MUST NOT: put `input` / `output` / `store` on a component; MUST NOT name a package a subprogram

**CONSTRAINT:** Catalog field names are `input`, `output`, `store` on subprograms; `runs` / `actuates` on opRuns. MUST NOT invent mint, writes-as-subprogram-fields, job-as-opRun-type, or aggregate-as-subprogram.

- MUST: use the field names in `catalog/types.go`
- MUST NOT: add `mint`, `mints`, `writes` on `Subprogram`; MUST NOT call a subprogram a feature, aggregate, or actuator

Enforcement: YAML/Go compile against `catalog` types; `ValidateStructure` unknown keys are dropped by YAML unmarshal (they never validate)
Violation: STOP, rename to the typed fields, re-validate

CORRECT: `input` / `output` / `store` / `runs` / `actuates` as in `testdata/tiny-module/architecture/typology.yaml`

PROHIBITED: `mint: invoice` on a subprogram; `mints: invoice` on an opRun

**CONSTRAINT:** After any catalog edit, MUST run structure validation before claiming done.

- MUST: `catalog.ValidateStructure()` in Go, or `typology validate REPO` (see [cli/SKILL.md](../cli/SKILL.md))
- MUST NOT: ship YAML that only parses

Enforcement: `go test` in this module; `typology validate` on the consumer repo
Violation: STOP, fix issues, re-run validate

## Go library

```go
t, err := catalog.LoadYAML("architecture/typology.yaml")
if err != nil { return err }
if issues := t.ValidateStructure(); len(issues) > 0 {
    return fmt.Errorf("catalog: %v", issues)
}
if err := catalog.SaveYAML("architecture/typology.yaml", t); err != nil {
    return err
}
```

Repo-path and import checks: `validate.Run` (`github.com/behaviorengineering/typology/validate`). One-slice agent protocol: `remediate.Run`.

`Component.Ops` is unused. MUST NOT set it or branch on it.

JSON/CALM export is not implemented. MUST NOT emit a second catalog format.

## Pre-completion checklist

- [ ] **Owners resolve:** every subprogram, actuator, and opRun `ownerComponent` is in slice `owns`
      Method: `ValidateStructure` has no `ownerComponent` issues
      Pass: empty structure issues for owners
      Fail: STOP, fix owner ids
- [ ] **Runs vs actuates:** no opRun sets both; named ids exist on the slice
      Method: structure issues for `runs` / `actuates`
      Pass: none
      Fail: STOP, split or add the program
- [ ] **Actuators complete:** signals and emits non-empty; ids distinct from subprograms
      Method: structure issues for actuators
      Pass: none
      Fail: STOP, fill or rename
- [ ] **Bindings:** cross-slice component edges have a SliceBinding
      Method: structure issues for bindings
      Pass: none
      Fail: STOP, add sliceBindings
- [ ] **Nouns:** packages in `owns`; programs in `subprograms`; emits in `actuators`; invocations in `opRuns`
      Method: read the slice YAML against the model table
      Pass: each row fits one type
      Fail: STOP, move the field
