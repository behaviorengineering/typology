---
name: typology-cable-board
description: >-
  Manage the Typology cable board: generate and read the portable package
  wiring graph (CLI: typology assembly-graph), open the interactive cable board
  viewer, interpret hubs, leaves, roles, import cables, and wrong-way edges, and
  steer AI implementation so new imports stay intentional. Load when the
  operator says cable board, assembly graph, package wiring, import cables,
  wrong-way imports, hubs/leaves, or asks to implement against observed wiring.
---

# Typology cable board

**Moral:** The cable board is the observed package wiring board. Packages are posts. Import edges are cables. Roles and layers say which way a cable may run. The catalog is intent; the cable board is what the code actually wires. Implement against both: match catalog ownership, then refuse new wrong-way cables unless the operator accepts the debt.

**CLI command (stable id):** `typology assembly-graph` · **Default artefact:** `tmp/typology/assembly-graph.json` · **Library:** `assemblygraph/` · **Viewer:** [`viewer/cable-board/`](../../../viewer/cable-board/) · **Catalog skill:** [catalog/SKILL.md](../catalog/SKILL.md) · **CLI skill:** [cli/SKILL.md](../cli/SKILL.md) · **First map:** [journey/SKILL.md](../journey/SKILL.md)

Human and agent name: **cable board**. File and command names stay `assembly-graph` until a deliberate rename ships. The interactive UI lives under `viewer/cable-board/` and consumes that JSON; it is not a second CLI command.

## When to load

- Operator says cable board, assembly board, assembly graph, package wiring, or asks to open the board viewer
- Before adding or changing package imports across module internals
- When diagnosing hubs, leaves, sole importers, or wrong-way layer edges
- During journey cluster-pass when the graph must explain merge candidates
- When implementing code that must respect observed roles (entrypoint, surface, aggregator, dto, config)

MUST NOT use this skill as a substitute for [catalog/SKILL.md](../catalog/SKILL.md) when editing slice YAML shape. MUST NOT use it as a substitute for [journey/SKILL.md](../journey/SKILL.md) when walking or freezing slices.

## Vocabulary

| Term | Meaning |
|------|---------|
| Cable board | The portable wiring document (JSON), the interactive viewer, and the practice of reading and implementing against them |
| Viewer | React board under `viewer/cable-board/` that loads `assembly-graph.json` (detangle on select, layers, wrong-way marks) |
| Post / node | One package (`path`, degrees, role, layer, hub/leaf flags) |
| Cable / edge | One directed import (`source` → `target`), optionally with `roleKind` |
| Wrong-way | A cable that climbs outward in role layer (inner imports outer) or a dto/config/observability package importing a surface/entrypoint/aggregator |
| Hub | High fan-in or fan-out package (board marks `isHub`) |
| Leaf | Package with no outbound imports inside the board (`isLeaf`) |
| Role | Observed package role from evidence (`entrypoint`, `http_surface`, `aggregator`, `adapter`, `exec_runner`, `dto`, `config`, `observability`, `unknown`) |
| Slice board | Cable board projected through one catalog slice: owned packages plus boundary stubs |
| Boundary stub | Lightweight node for a package outside the selected slice (`isBoundary`, `boundaryKind`, `ownerId`) |
| Missing binding | Boundary cable with no matching `sliceBindings` / allowed `componentBindings` (`bindingStatus: missing`) |

## Commands

```text
typology assembly-graph REPO [--module PATH] [--catalog PATH] [--slice SLICE|--all-slices] [--out PATH|--out-dir DIR]
```

Defaults: writes `REPO/tmp/typology/assembly-graph.json`. Use `--module PATH` in a multi-module workspace the same way as discover/validate. With `--slice`, the board keeps only that slice's owned packages plus boundary stubs. With `--all-slices`, each catalog slice is written under `--out-dir` (default `tmp/typology/boards/<slice>/assembly-graph.json`). Library equivalent: `assemblygraph.Build`, optional `assemblygraph.Project`, then `assemblygraph.WriteJSON`.

Companion topology view (text, not the cable-board JSON): `typology show graph REPO` (see [cli/SKILL.md](../cli/SKILL.md)).

Interactive viewer (human surface for the same JSON):

```text
# from Typology module root
make build
make cable-board-sample
cd viewer/cable-board && npm install && npm run dev
```

One server serves many named boards. Register a board without touching the
others (stable id: lowercase, digits, hyphens):

```text
./viewer/cable-board/scripts/load-graph.sh REPO BOARD_ID [--module PATH] [--label TEXT] [--make-default]
./viewer/cable-board/scripts/load-graph.sh REPO BOARD_ID --slice SLICE [--catalog PATH] [--module PATH] [--label TEXT]
./viewer/cable-board/scripts/load-graph.sh REPO --all-slices [--catalog PATH] [--module PATH]
```

Then open `http://localhost:5173/?board=BOARD_ID` in one window and another
board id in a second window. The header switcher moves between boards and
keeps the board in the URL. Registry file: `viewer/cable-board/public/boards.json`
(`defaultBoard` plus `boards[]`). Only the tiny-module sample is committed;
consumer boards stay local. Optional URL override: `http://localhost:5173/?src=/assembly-graph.json`.

Prefer slice boards for human review when a full-module board is dense: use the
confirmed catalog slice id as the board id (`?board=chronology`). Boundary stubs
are dashed cards; amber dashed cables mean missing catalog bindings.

Or copy an existing board file:

```text
typology assembly-graph REPO --out $MOD/viewer/cable-board/public/assembly-graph.json
```

`$MOD` is `go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology` when Typology is only a dependency.

## Steps

1. **Scope:** resolve the repository root and optional `--module` the same way as [cli/SKILL.md](../cli/SKILL.md). MUST NOT scan every `go.work` module by accident.
2. **Generate:** prefer `typology assembly-graph REPO --catalog PATH --slice SLICE` (or `--all-slices`) when the catalog is confirmed. Use the raw module harvest only when comparing the whole module. Confirm stdout reports owned nodes, boundary stubs, and missing bindings for slice boards.
3. **Open the viewer (when a human needs the board):** register with `viewer/cable-board/scripts/load-graph.sh REPO BOARD_ID` (use the slice id as BOARD_ID for slice boards) and run `npm run dev` from `viewer/cable-board`. Agents MAY skip the viewer when only machine checks are required, but MUST still regenerate and read the JSON.
4. **Read the board:** open `http://localhost:5173/?board=BOARD_ID`. Inventory hubs, leaves, boundary stubs, every edge with `wrongWay: true`, and every edge with `bindingStatus: "missing"`.
5. **Align with catalog:** map boundary debt back to `sliceBindings` / `componentBindings` / `owns` per [catalog/SKILL.md](../catalog/SKILL.md). Refine the catalog, regenerate, re-open the board.
6. **Implement:** when adding imports, prefer cables that stay inward on role layer and that match declared bindings. After the change, regenerate the board and re-check wrong-way and missing-binding counts.
7. **Debt:** if a wrong-way or missing-binding cable must remain, record it in `.typology/typology-journey.md` under technical debt (or the consumer's agreed debt log) with target refactoring. MUST NOT silently accept new boundary debt.

## Core constraints

**CONSTRAINT:** Agents MUST call the artefact a cable board in operator-facing prose, while using the stable CLI id `assembly-graph` in commands and paths.

- MUST: say "cable board" when tutoring or summarizing wiring
- MUST: run `typology assembly-graph` (not an invented `cable-board` command) until a rename ships
- MUST NOT: invent a second JSON format or a parallel command name in scripts

Enforcement: skill load; CLI help lists `assembly-graph`
Violation: STOP, correct the command to `typology assembly-graph`, keep human wording as cable board

CORRECT:
```text
Regenerate the cable board: typology assembly-graph . --module engine --slice chronology
```

PROHIBITED:
```text
typology cable-board .
```

**CONSTRAINT:** When a confirmed catalog exists, human-facing boards SHOULD be slice-scoped. Every human-facing board MUST have a stable id (prefer the catalog slice id).

- MUST: use `--slice` / `--all-slices` (or loader equivalents) for review boards when slices are confirmed
- MUST: keep boundary stubs for external callers and dependencies; MUST NOT drop cross-slice cables from the projection
- MUST NOT: treat the full-module board as the default human view when slice boards are available

Enforcement: skill steps + viewer README
Violation: STOP, project with `--slice`, register `?board=<slice-id>`

**CONSTRAINT:** When the operator asks to *see* or *open* the cable board, agents MUST use the shipped viewer under `viewer/cable-board/` with a registered board id. MUST NOT invent a second graph format or point humans only at raw JSON when a visual walk is requested.

- MUST: regenerate with `typology assembly-graph`, then register with `viewer/cable-board/scripts/load-graph.sh REPO BOARD_ID` (stable id per repo scope)
- MUST: give the human a `?board=BOARD_ID` URL; open a second window with a different board id when comparison is requested
- MUST: run the Vite app from that directory (`npm install && npm run dev`) when a human needs the interactive board
- MUST NOT: claim the viewer is unavailable while this module tree contains `viewer/cable-board/`

Enforcement: viewer README + skill steps; board id present in `public/boards.json`
Violation: STOP, open `viewer/cable-board/README.md`, register the board, start `npm run dev`

CORRECT:
```text
./viewer/cable-board/scripts/load-graph.sh . chronology --module engine --slice chronology
cd "$MOD/viewer/cable-board" && npm install && npm run dev
# open http://localhost:5173/?board=chronology
```

PROHIBITED:
```text
Here is the JSON path only; there is no UI for cable boards.
```

**CONSTRAINT:** Before claiming an implementation that adds internal imports is done, MUST regenerate the cable board and inspect wrong-way edges that touch the changed packages.

- MUST: run `typology assembly-graph` after meaningful import edits
- MUST: list new or remaining wrong-way cables that involve edited packages
- MUST NOT: claim clean wiring while those edges stay unexplained

Enforcement: compare wrong-way count and edge ids before vs after; read `wrongWayReason`
Violation: STOP, fix the import direction or record debt with operator approval

CORRECT: after adding `internal/billing/store` → `internal/ledger`, regenerate board; confirm no new wrong-way on those ids

PROHIBITED: merge a PR that adds `internal/board` → `internal/server` without regenerating or explaining the wrong-way mark

**CONSTRAINT:** Cable board is observation. Catalog is intent. MUST NOT treat the JSON as permission to ignore slice ownership or bindings.

- MUST: update catalog ownership or bindings when the intended architecture changes
- MUST: load [catalog/SKILL.md](../catalog/SKILL.md) before editing YAML
- MUST NOT: "fix" a wrong-way finding by rewriting the JSON by hand

Enforcement: catalog validate + regenerated board; JSON is always regenerated output
Violation: STOP, discard hand-edited JSON, fix code or catalog, regenerate

CORRECT: move a package under the owning slice in YAML, then implement the import the binding allows

PROHIBITED: delete a `wrongWay` flag in `assembly-graph.json` to clear a finding

**CONSTRAINT:** Wrong-way cables are fail-loud signals for agents, not optional style nits.

- MUST: treat `wrongWay: true` as a blocking finding for any cable introduced in the current change set unless the operator explicitly accepts debt
- MUST: prefer fixing import direction (outer depends on inner) over layering exceptions
- MUST NOT: add dto/config/observability → entrypoint/http_surface/aggregator imports

Enforcement: board `wrongWay` / `wrongWayReason`; role layers in `assemblygraph`
Violation: STOP, reverse the dependency or extract a shared inner package; regenerate

CORRECT: HTTP surface imports aggregator; aggregator fills dto

PROHIBITED: dto package imports HTTP surface to "reuse a handler type"

**CONSTRAINT:** Multi-module boards MUST pass `--module` (or rely on catalog `scope.modules` via the same CLI conventions as discover) so the board matches the catalog scope under change.

- MUST: pass `--module PATH` when the workspace has multiple Go modules and the catalog scopes one of them
- MUST NOT: generate an unbounded board and use it to justify edits outside the active scope

Enforcement: CLI `--module`; catalog `scope.modules`
Violation: STOP, regenerate with the correct module path

CORRECT: `typology assembly-graph . --module engine`

PROHIBITED: board the whole monorepo, then refactor an out-of-scope UI module from engine findings

## Board JSON (contract)

Top level: `nodes`, `edges`, optional `slice` (set on projected boards).

Node fields agents MUST read: `id`, `path`, `inDegree`, `outDegree`, `imports`, `importedBy`, `isHub`, `isLeaf`, `layer`, optional `role`, `roleConfidence`, and on stubs `isBoundary`, `boundaryKind`, `ownerId`.

Edge fields agents MUST read: `id`, `source`, `target`, optional `roleKind`, `wrongWay`, `wrongWayReason`, and on boundary cables `bindingStatus`, `boundaryKind`.

Contract smoke: `scripts/check-assembly-graph.py` in this module.

## Pre-completion checklist

- [ ] **Generated:** `typology assembly-graph` wrote JSON for the scoped repo/module
      Method: file exists; CLI stdout shows node/edge counts
      Pass: non-empty `nodes` and `edges`
      Fail: STOP, fix harvest/CLI errors, regenerate
- [ ] **Viewer when requested:** if the operator asked to see the board, the Vite viewer is running on current JSON
      Method: the board id is registered in `viewer/cable-board/public/boards.json`; `http://localhost:5173/?board=ID` loads
      Pass: human can open the named board, or operator waived the UI this turn
      Fail: STOP, register the board with `scripts/load-graph.sh REPO BOARD_ID` and start `npm run dev`
- [ ] **Wrong-way reviewed:** every `wrongWay` edge that touches this change is fixed or logged as debt
      Method: filter edges with `wrongWay: true`
      Pass: none new unexplained, or debt rows exist
      Fail: STOP, fix imports or record debt with operator approval
- [ ] **Catalog aligned:** cross-slice cables have sliceBindings (and componentBindings when required)
      Method: [catalog/SKILL.md](../catalog/SKILL.md) + `typology validate` when a catalog exists
      Pass: validate clean or debt logged
      Fail: STOP, add bindings or revert the cable
- [ ] **No hand-edited board:** JSON was regenerated, not patched
      Method: only CLI/library write path used
      Pass: no manual JSON surgery
      Fail: STOP, regenerate from source
