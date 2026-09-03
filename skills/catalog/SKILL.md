---
name: typology-catalog
description: >-
  Author Typology catalogs (YAML or catalog.Typology in Go): slices, components,
  surfaces, subprograms, actuators, opRuns, sliceBindings, componentBindings. Load when
  writing .typology/typology.yaml, calling catalog.LoadYAML or SaveYAML, or
  mapping a product contract onto the Typology model.
---

# Typology catalog

**Moral:** A slice is a bounded context. A component is a package. A surface is a built interaction artefact (UI, CLI, or API) that owns packages. A subprogram is a standing program. An opRun is one gated invocation. An actuator is a signal-triggered emit. Do not collapse those five.

**Types:** `catalog/types.go` · **Fixture:** `testdata/tiny-module/.typology/typology.yaml` · **I/O:** `catalog.LoadYAML`, `catalog.SaveYAML`, `catalog.ValidateStructure`

## When to load

- Adding or editing `.typology/typology.yaml` (or `typology.yaml` at repo root)
- Building a `catalog.Typology` in Go and writing it with `SaveYAML`
- Naming subprograms, actuators, opRuns, or bindings
- A validate error about `runs`, `actuates`, `ownerComponent`, `signals`, or `emits`

## Model

| Type | Role |
|------|------|
| `Typology` | Whole map (`id`, optional `scope.modules`, `slices`, optional bindings) |
| `Slice` | Bounded context with a required business `objective` (`owns`, `surfaces`, `opRuns`, `subprograms`, `actuators`, `docs`) |
| `Component` | Package path. Domain packages live on `owns[]`. Interaction packages live under a `Surface`. |
| `Surface` | Built interaction artefact (`kind`: `ui`, `cli`, or `api`) with nested `components[]` (id + path only). |
| `OpRun` | One gated invocation (CLI, HTTP, human, signal, or later schedule). Optional `runs` or `actuates`, not both. |
| `Subprogram` | Standing program: required `objective` (business why), `input`, `output`, optional `store`, `gate`. Origin for `store` paths when first written. |
| `Actuator` | Signal in, emit out (usually past the instance edge). Requires `objective`, `signals`, and `emits`. |
| `SliceBinding` | Slice to slice: `consumes` or `reads` |
| `ComponentBinding` | Component to component: `must`, `must_not`, or `reads` |

Gates: `auto` | `test` | `human`.

Default catalog path: `.typology/typology.yaml`. Default docs root: `docs/develop`.

**Scope:** One repository owns one catalog and its architecture docs. In a multi-module workspace, set `scope.modules` to repository-relative module roots such as `engine` or `ui/viewer`. `go.work` does not define Typology scope.

**CONSTRAINT:** Every slice MUST set a non-empty `objective` that states the bounded context's business purpose.

- MUST: write one plain-English objective clause for every slice
- MUST NOT: omit `objective` or use a generic placeholder that gives no business purpose

Enforcement: `catalog.ValidateStructure` (`missing objective`)
Violation: STOP, write the slice's business why, then re-validate

CORRECT:
```yaml
slices:
  - id: billing
    objective: Operate billing records for fixture tests.
```

PROHIBITED:
```yaml
slices:
  - id: billing
    owns:
      - id: billing-store
        path: internal/billing/store
        layer: domain
    # missing objective
```

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
    objective: Mint an invoice record from a store request.
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

**CONSTRAINT:** Every subprogram and actuator MUST set a non-empty `objective` (the business why). Docs FILL quotes this field; do not leave it blank for the docs agent to invent.

- MUST: one plain-English objective clause per subprogram and actuator
- MUST NOT: omit `objective`; MUST NOT paste Input/Output into `objective`

Enforcement: `ValidateStructure` (`subprogram … missing objective`; `actuator … missing objective`)
Violation: STOP, write the why from product Must / operator job, re-validate

CORRECT:
```yaml
subprograms:
  - id: concept
    ownerComponent: review-core
    objective: Mint stable cn_ ids only after human Accept so proposals never enter Neo4j.
    input: human Accept on concept proposal
    output: cn_ from seed.yaml after Accept
    gate: human
```

PROHIBITED:
```yaml
subprograms:
  - id: concept
    ownerComponent: review-core
    input: human Accept
    output: cn_
    # missing objective
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

**CONSTRAINT:** Actuator id MUST NOT collide with a subprogram id on the same slice. Actuator MUST set non-empty `objective`, `signals`, and `emits`.

- MUST: unique id vs subprograms; non-empty objective; at least one signal; at least one emit
- MUST NOT: reuse a subprogram id; MUST NOT leave objective, signals, or emits empty

Enforcement: `ValidateStructure` (`id already used by a subprogram`; `missing objective`; `missing signals`; `missing emits`)
Violation: STOP, rename or fill fields, re-validate

CORRECT:
```yaml
actuators:
  - id: invoice-webhook
    ownerComponent: billing-http
    objective: Notify external systems when an invoice is minted.
    signals: [invoice.minted]
    emits: [webhook]
    gate: auto
```

PROHIBITED:
```yaml
actuators:
  - id: invoice-webhook
    ownerComponent: billing-http
    signals: [invoice.minted]
    emits: [webhook]
    # missing objective
```

```yaml
subprograms:
  - id: notify
actuators:
  - id: notify          # collision
    objective: send notify
    signals: [done]
    emits: [webhook]
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

**CONSTRAINT:** Docs are a tree, not a flat six-pack. `DocPageKind` files are leaves. Extra program pages are not a seventh kind.

- MUST: human nav is slice → Overview → Owns → Subprograms → Surfaces (CLI, UI, API, Jobs)
- MUST: `docs.pages[]` lists only pages that exist; discover/emit default to overview+components, plus contracts/cli/presentation when those surface kinds exist, plus pipelines when `opRuns` is non-empty
- MUST: each `subprograms[].id` has `docs/develop/<slice>/subprograms/<id>.md`; each `actuators[].id` has `docs/develop/<slice>/actuators/<id>.md`
- MUST NOT: treat Jobs as a `surfaces[].kind`; Jobs is the pipelines DocPage under Surfaces
- MUST NOT: require all six `DefaultDocPageKinds` when the slice has no CLI, UI, API, or opRuns

Enforcement: `catalog.DefaultDocCluster`; `validate` program-page paths; emit README hub
Violation: STOP, trim `docs.pages` or add the missing leaf, re-validate

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

**CONSTRAINT:** Slices MUST be true bounded contexts, not temporal pipeline stages, capabilities, or horizontal technical tiers.

- MUST: group packages by shared domain lifecycle and aggregates (e.g. `review`, `context`, `operations`)
- MUST NOT: create separate slices for sequential workflow steps (e.g. `staging` → `orchestrate` → `judge` → `publish` belong in one `review` bounded context)
- MUST NOT: elevate internal capabilities (DSPy eval, LLM gateway, workspace inspection) into standalone domain pillars
- MUST NOT: create horizontal technical slices (e.g. `cli` or `platform`); place CLI packages on `surfaces[kind: cli]` of the domain slice they invoke
- MUST NOT: merge packages based on stem similarity alone (e.g. `agent` dispatch vs `agenting` grounding) without verifying shared importers and domain purpose
- MUST: keep platform utility leaves (`config`, telemetry, auth) small and shared rather than swallowed into an arbitrary domain hub

**CONSTRAINT:** Catalog field names are `input`, `output`, `store` on subprograms; `runs` / `actuates` on opRuns. MUST NOT invent mint, writes-as-subprogram-fields, job-as-opRun-type, or aggregate-as-subprogram.

- MUST: use the field names in `catalog/types.go`
- MUST NOT: add `mint`, `mints`, `writes` on `Subprogram`; MUST NOT call a subprogram a feature, aggregate, or actuator

Enforcement: YAML/Go compile against `catalog` types; `ValidateStructure` unknown keys are dropped by YAML unmarshal (they never validate)
Violation: STOP, rename to the typed fields, re-validate

CORRECT: `input` / `output` / `store` / `runs` / `actuates` as in `testdata/tiny-module/.typology/typology.yaml`

PROHIBITED: `mint: invoice` on a subprogram; `mints: invoice` on an opRun

**CONSTRAINT:** After any catalog edit, MUST run structure validation before claiming done.

- MUST: `catalog.ValidateStructure()` in Go, or `typology validate REPO` (see [cli/SKILL.md](../cli/SKILL.md))
- MUST NOT: ship YAML that only parses

Enforcement: `go test` in this module; `typology validate` on the consumer repo
Violation: STOP, fix issues, re-run validate

## Go library

```go
t, err := catalog.LoadYAML(".typology/typology.yaml")
if err != nil { return err }
if issues := t.ValidateStructure(); len(issues) > 0 {
    return fmt.Errorf("catalog: %v", issues)
}
if err := catalog.SaveYAML(".typology/typology.yaml", t); err != nil {
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
- [ ] **Actuators complete:** objective, signals, and emits non-empty; ids distinct from subprograms
- [ ] **Subprogram/actuator why:** every listed program has `objective` (business why for docs FILL)
      Method: structure issues for actuators
      Pass: none
      Fail: STOP, fill or rename
- [ ] **No papered-over boundary debt:** unowned packages or false aggregate claims are logged in journey technical debt table rather than artificially assigned
      Method: compare subprogram package implementations against slice owns/surfaces
      Pass: clean boundaries, or debt explicitly logged in journey file
      Fail: STOP, log debt row or fix ownership
- [ ] **Bindings:** cross-slice component edges have a SliceBinding
      Method: structure issues for bindings
      Pass: none
      Fail: STOP, add sliceBindings
- [ ] **Nouns:** packages in `owns`; programs in `subprograms`; emits in `actuators`; invocations in `opRuns`
      Method: read the slice YAML against the model table
      Pass: each row fits one type
      Fail: STOP, move the field
