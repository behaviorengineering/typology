---
name: typology-journey
description: >-
  First-map and refine journey for Typology in a new or existing Go repo:
  write .typology/typology-journey.md from the plan template, discover a
  draft catalog, tutor the operator through one slice per turn, freeze the
  as-is map, refine desired architecture, emit and validate, then fill
  develop DocPages. Load when typology lands, onboard, first catalog, first
  map, discovery plan, walk slices, write slice docs, or resume an unfinished
  typology-journey.md.
---

# Typology journey

**Moral:** Discover drafts. A journey file is the session. The operator learns the as-is map before anyone treats cluster ids as architecture. Emit waits for commit.

**CLI skill:** [cli/SKILL.md](../cli/SKILL.md) · **Catalog skill:** [catalog/SKILL.md](../catalog/SKILL.md) · **Docs skill:** [docs/SKILL.md](../docs/SKILL.md) · **Plan template:** [plan-template.md](plan-template.md)

Default paths (consumer repo root `REPO`):

| Artefact | Path |
|----------|------|
| Journey (session) | `REPO/.typology/typology-journey.md` |
| Draft catalog | `REPO/tmp/typology/typology.yaml` (default; override with `--out`) |
| Confirmed catalog | `REPO/.typology/typology.yaml` |

## When to load

- Typology just landed in a repo (first catalog, onboard, first map)
- `.typology/typology-journey.md` exists and Status.phase is not `done`
- Operator asks to walk slices, understand the current map, refine desired architecture, or write slice docs

Load [cli/SKILL.md](../cli/SKILL.md) for commands. Load [catalog/SKILL.md](../catalog/SKILL.md) when filling programs or bindings. Load [docs/SKILL.md](../docs/SKILL.md) when Status.phase is `docs` or the operator asks to fill develop DocPages.

MUST NOT use this skill as the primary load when the operator only asked to `validate`, `emit`, or `remediate` a confirmed catalog with no journey in progress.

## Core constraints

**CONSTRAINT:** The journey file is the only session state. Create it from the template before discover. Resume from it on every later turn.

- MUST: copy [plan-template.md](plan-template.md) to `.typology/typology-journey.md` when that file is missing and this skill loaded for a first map or refine
- MUST: Read the journey file first on every turn; update Status, Resume, and the checkbox that this turn closed
- MUST NOT: keep progress only in chat; MUST NOT start a second journey file under another name

Enforcement: `.typology/typology-journey.md` exists; Resume.phase matches the next pending checkbox
Violation: STOP, create or repair the journey file from the template, copy any known decisions into it, then continue

CORRECT: first turn writes `.typology/typology-journey.md` with Status.phase `land`; next chat Reads it and continues at the first pending slice

PROHIBITED: `typology discover .` into chat with no journey file, then asking the operator to "confirm slices"

**CONSTRAINT:** Discover writes a draft. The confirmed catalog is untouched until commit.

- MUST: `typology discover REPO` (writes the draft to `REPO/tmp/typology/typology.yaml`) when there is no confirmed catalog and no draft
- MUST: `typology discover REPO --out PATH` when a specific draft path is required; default draft path is `REPO/tmp/typology/typology.yaml`
- MUST: if `.typology/typology.yaml` already exists, copy it to the draft path when the draft is missing, then skip discover unless the operator explicitly asked to rediscover
- MUST NOT: `typology discover REPO --out .typology/typology.yaml` (overwrites the confirmed catalog before `commit`)
- MUST NOT: write `.typology/typology.yaml` before Status.phase is `commit`

Enforcement: draft path in the discover command; git status of `.typology/typology.yaml` unchanged through phases `land` .. `desired`
Violation: STOP, restore the confirmed catalog from git if overwritten; rediscover to the draft path if a draft is still needed

CORRECT: `typology discover .` writes the draft to `tmp/typology/typology.yaml` by default

PROHIBITED: `typology discover . --out .typology/typology.yaml` onto a confirmed `.typology/typology.yaml`

**CONSTRAINT:** Cluster pass consolidates mechanical 1:1 package clusters before slice walk.

- MUST: run `typology show graph REPO` (or `--suggest-merges`) to inspect coupling degrees, leaves, hubs, and sole importers before walking individual slices
- MUST: apply merge heuristics:
  1. **Sole importer:** package imported by only one caller (e.g. `cluster` only imported by `staging`) -> propose merge into caller
  2. **Same job family:** companion packages around one domain concern (e.g. `contextstore` + `contextdigest` + `contextgate`) -> propose merge into family slice (`context`)
  3. **Split companion packages:** `sa` + `satools` -> `sa`
  4. **Manifest/CLI companions:** `diff` used only for staging manifests -> `staging`
  5. **Forge side-effects:** `outbound` + `status` next to `publish` -> `publish` or `forge`
  6. **Projection / Visualizer packages:** package only reads domain models to emit graph, tree, or UI view models (e.g. `graphwiring` over `concept` / `ontology`) -> attach as `surfaces[]` or merge into the entity domain slice
- MUST: enforce DDD anti-pattern checks:
  - **Anti-pattern 1: Temporal pipeline stages as slices.** Sequential execution phases (prep -> wave -> judge -> publish) share the same aggregate lifecycle and belong in ONE bounded context (e.g. `review`), not separate peer slices.
  - **Anti-pattern 2: Capabilities as domain pillars.** Internal facilities (DSPy eval, LLM gateways, workspace inspection) are capabilities invoked by workflows, not autonomous domain pillars.
  - **Anti-pattern 3: Horizontal technical tiers as slices.** Having a standalone `cli` or `platform` slice fragments the domain. CLI commands belong on `surfaces[]` of the domain slice they invoke; root entry binary belongs under host/operations boundary.
  - **Anti-pattern 4: Projection / Visualizer as a standalone slice (`projection-as-slice`).** A package that exists solely to project, query, or render a graph, tree, or dashboard from domain entities defined elsewhere has no aggregate lifecycle of its own; it belongs on `surfaces[]` of the underlying entity domain slice, not as an independent peer slice.
  - **Platform leaves:** keep platform utility leaves (`config`, telemetry, auth) small and separate rather than swallowing them into the first domain hub.
- MUST NOT: trust name similarity alone without checking importers (`agent` dispatch vs `agenting` grounding have different callers and represent different contexts)
- MUST: present candidate merge clusters with one-line rationale to the operator and obtain approval before seeding the slice-walk table

Enforcement: candidate clusters approved in `.typology/typology-journey.md`; walk table seeded with consolidated slices
Violation: STOP, run cluster pass, tutor merge candidates, wait for operator gate

**CONSTRAINT:** Boundary violations and technical debt MUST be recorded in the journey file rather than papered over.

- MUST: during `cluster-pass` and `desired`, audit declared subprograms and actions against actual package import dependencies
- MUST: identify false aggregate ownership (e.g. a slice claiming an autonomous mining or evaluation subprogram whose underlying engine packages it does not own)
- MUST: identify UI/domain conflations (e.g. treating an entity mining engine as part of a review UI slice merely because its triage card mounts on that page)
- MUST: check for unmapped orphan packages across the module
- MUST: record every detected violation in the `## Technical debt & boundary violations` table in `.typology/typology-journey.md` with a clear target refactoring
- MUST NOT: paper over boundary violations by artificially attributing unowned packages or subprograms to unrelated slices without logging the debt

Enforcement: `Technical debt & boundary violations` table in `.typology/typology-journey.md` populated when violations exist
Violation: STOP, audit boundary tensions, record debt rows, tutor operator on refactoring targets

**CONSTRAINT:** Slice walk is one proposed slice per turn. The assistant reads; the operator gates.

- MUST: during Status.phase `slice-walk`, present exactly one pending slice from the journey table
- MUST: end that turn with the five choices below and wait
- MUST NOT: dump the full draft YAML; MUST NOT ask the operator to confirm every slice in one message

Enforcement: journey table has at most one row Status changed this turn (except bulk Later the operator named)
Violation: STOP, revert extra slice edits, finish the open slice gate

CORRECT: walk `billing` this turn; `ledger` stays `pending`

PROHIBITED: pasting every `owns` block from the draft and asking "looks good?"

**CONSTRAINT:** Every human gate uses tutor framing. Jargon gets a gloss in the same breath.

- MUST: before keep / rename / merge / split / later (and before situation-freeze, commit confirm, or a docs-page gate), write four beats: situation, why it matters, what the draft already shows, then the numbered choices
- MUST: gloss `slice` as bounded context, `owns` as packages, `binding` as coupling, on first use in that turn
- MUST NOT: drop a bare "keep this slice?"; MUST NOT depend on Consilium tutor skills (this module is portable)

Enforcement: the message that asks for a gate contains those four beats
Violation: STOP, rewrite the gate with the four beats, do not edit the draft until the operator answers

CORRECT:

```markdown
We are on the first map, walking proposed slice `billing` from the import-graph draft.
A package cluster is not yet a bounded context you chose; your call is what we keep in the catalog.
The draft gives `billing` three packages (`internal/billing/store`, `internal/billing/httpapi`, `internal/billing/cli`) and a `reads` binding to `ledger`.
Your call: (1) keep (2) rename (3) merge into another slice (4) split (5) later.
```

PROHIBITED: `Confirm slice billing?`

**CONSTRAINT:** Situation and desired are different phases. Emit waits for commit. Docs fill waits for emit. `done` waits for docs.

- MUST: finish slice-walk and an explicit situation-freeze before Status.phase `desired`
- MUST: run `typology emit` only while Status.phase is `commit`
- MUST: after emit and validate succeed, set phase `docs` (not `done`) and load [docs/SKILL.md](../docs/SKILL.md)
- MUST: set phase `done` only when every Docs row is `done` or `skip-none` (`later` rows allowed)
- MUST NOT: emit DocPages or claim the catalog is the architecture during `land`, `situation-draft`, `slice-walk`, or `situation-freeze`
- MUST NOT: fill subprograms as a way to skip the as-is walk
- MUST NOT: fill develop DocPages before phase `docs`

Enforcement: Status.phase in the journey file; emit only in `commit`; Docs table complete before `done`
Violation: STOP, revert emit outputs if the operator did not confirm commit; return to the pending phase

CORRECT: freeze as-is, then in `desired` rename slices and add programs, then `commit` copies the draft and emits, then `docs` fills one page per turn

PROHIBITED: `typology emit .` on the raw discover draft before the operator has walked a slice

## Slice walk choices

End each slice-walk turn with:

```markdown
1. **Keep**: this cluster is a bounded context; leave the slice id.
2. **Rename**: same cluster, new id (say the id).
3. **Merge**: not its own context; name the slice it belongs to.
4. **Split**: mixed contexts; say how to cut the packages.
5. **Later**: skip this slice this session; leave Status `later`.
```

After the operator picks:

| Pick | Agent does |
|------|------------|
| Keep | Set row Status `keep`. Next pending slice next turn. |
| Rename | Set row Status `rename`; change the slice id in the draft YAML; note the old id. |
| Merge | Set row Status `merge`; move `owns` into the named slice in the draft; drop the empty slice. |
| Split | Set row Status `split`; add the new slice ids as `pending` rows; split `owns` in the draft. |
| Later | Set row Status `later`. Next pending slice next turn. |

MUST NOT apply rename / merge / split until the operator names the target ids. MUST load [catalog/SKILL.md](../catalog/SKILL.md) before editing draft YAML shape.

## Steps

1. **Resume or create**: if `.typology/typology-journey.md` exists, Read it and obey Resume. If missing, copy [plan-template.md](plan-template.md) there, fill Status (repo, date, phase `land`), then continue.
2. **Land**: confirm `typology version` (or `make build` in this module) works. Tick Land checkboxes. Set phase `situation-draft`.
3. **Situation draft**: discover or copy per the discover constraint (`typology discover REPO`, which writes `REPO/tmp/typology/typology.yaml`). Tick Situation-draft checkboxes. Set phase `cluster-pass`.
4. **Cluster pass**: run `typology show graph REPO` (or `--suggest-merges`). Identify hubs, leaves, sole importers, and companion packages. Apply DDD anti-pattern checks (merge sequential pipeline stages, internal capabilities, and technical CLI tiers into bounded contexts). Present candidate clusters to operator with merge rationale. Upon approval, update the draft catalog and seed the walk table with consolidated slices. Set phase `slice-walk`.
5. **Slice walk**: one pending row per turn (tutor gate). Keep: no YAML edit. Rename/Merge/Split: edit the draft only after the operator names the target ids. When no `pending` rows remain (Later rows allowed), set phase `situation-freeze`.
6. **Situation freeze**: tutor the operator on the as-is map (slice ids, main bindings). Wait for an explicit freeze. Tick freeze checkboxes. Set phase `desired`.
7. **Desired**: reshape the draft toward the architecture they want (names, bindings, programs per catalog skill). Record each open decision in the journey file. When they say the draft is the catalog they want, set phase `commit`.
8. **Commit**: copy draft to `.typology/typology.yaml`. `typology emit REPO` (writes `.typology/README.md`, `tools.yaml`, and the `AGENTS.md` pointer). `typology validate REPO`. Fix every issue (cli skill). Tick commit checkboxes. Set phase `docs`. Build the Docs table from `docs.pages[]` plus `subprograms/` and `actuators/` leaves (all `pending`).
9. **Docs**: load [docs/SKILL.md](../docs/SKILL.md). One pending DocPage per turn. `done` only when every Docs row is `done` or `skip-none` (`later` allowed).
10. **Stop**: after each turn that asked a gate, update Resume and wait. MUST NOT chain the next slice or next DocPage in the same message. Land, situation-draft, and cluster-pass proposal MAY chain to the cluster gate. Commit MAY run emit and validate in the same turn as the first docs-page gate.

## Pre-completion checklist

This checklist is per turn, not whole-journey done.

- [ ] **Journey file:** `.typology/typology-journey.md` exists and Resume matches this turn
      Method: Read Status.phase and Resume.next
      Pass: file present; Resume names the unit just closed or the gate waiting
      Fail: STOP, write or repair the journey file
- [ ] **Draft path:** discover or catalog copies used `tmp/typology/typology.yaml` (or a `--out` override)
      Method: command line or copy destination
      Pass: draft path; confirmed yaml unchanged before phase `commit`
      Fail: STOP, restore confirmed catalog; move work to the draft
- [ ] **One unit:** this turn stopped at the first human gate (or closed commit with no remaining gate)
      Method: diff the journey file plus the outgoing message
      Pass: deterministic phases (land, situation-draft, commit emit/validate) MAY chain; at most one slice/freeze/desired/commit/docs question
      Fail: STOP, revert extra slice or phase jumps past a gate
- [ ] **Tutor gate:** if this turn asked the operator, the four beats are in the message
      Method: reread the outgoing gate
      Pass: situation, why, what we have, numbered choices
      Fail: STOP, rewrite the gate; do not edit the draft
- [ ] **No early emit:** emit only in phase `commit`
      Method: Status.phase vs commands run
      Pass: no emit before `commit`; no emit during `docs` onto filled pages
      Fail: STOP, revert emitted pages if they overwrote human docs; return to the pending phase
- [ ] **No early done:** phase `done` only after Docs rows are `done` or `skip-none`
      Method: Status.phase vs Docs table
      Pass: `done` implies no `pending` / `filled` / `revised` rows
      Fail: STOP, set phase `docs` and finish the table
