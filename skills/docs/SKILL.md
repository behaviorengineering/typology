---
name: typology-docs
description: >-
  Fill and evaluate Typology develop DocPages (overview, components, contracts,
  cli, presentation, pipelines): cold-read, evidence gate, one page per turn,
  then five-filter revise. Load when journey phase is docs, the operator asks
  to write slice docs, fill emit stubs, or evaluate docs/develop pages.
---

# Typology docs

**Moral:** Emit writes stubs. This skill fills and scores them. Catalog and repo are the source. Nav is a tree (Overview → Owns → Subprograms → Surfaces); the six kinds are leaves.

**Journey:** [journey/SKILL.md](../journey/SKILL.md) · **CLI:** [cli/SKILL.md](../cli/SKILL.md) · **Catalog:** [catalog/SKILL.md](../catalog/SKILL.md) · **Templates:** [reference.md](reference.md)

MUST NOT use this skill for README pitch, story, or counsel prose (Consilium `revise-flow` owns those). MUST NOT invent HTTP, CLI, UI, or pipeline surface the catalog does not name.

The human architecture brief at `docs/architecture/typology.md` is a separate
reviewed projection owned by the repository. Run `typology architecture REPO`
after the catalog is confirmed. In a multi-module repository it only inspects
the modules listed in `scope.modules`, not every module in `go.work`. It compares
catalog intent with observed Go topology and lists
findings, but it does not decide whether a design is correct. Read each
finding, fix the code or catalog when appropriate, and record unresolved
boundary debt in the journey file. Remove the generated marker only after a
human accepts the explanation.

## When to load

- Journey Status.phase is `docs`
- Operator asks to write slice docs, fill emit stubs, or evaluate `docs/develop/` pages
- A DocPage still has `<!-- typology:generated -->` after a confirmed catalog exists

Load [reference.md](reference.md) before the first cold-read. Load catalog skill when a page-kind contract needs YAML fields. Load cli skill only to re-emit or validate paths.

MUST NOT use this skill as the primary load when the operator only asked to discover, remediate, or reshape slice names (journey / cli / catalog).

## Core constraints

**CONSTRAINT:** The docs table is session state for this phase.

- MUST: when `.typology/typology-journey.md` exists, fill the Docs table from the confirmed catalog (`docs.pages[]`) before the first page turn
- MUST: if there is no journey file and the operator named one existing page, keep a one-off table in chat for that page only
- MUST NOT: track fill progress only in chat when a journey file exists; MUST NOT start a second docs tracker under another name

Enforcement: Docs table rows match catalog `docs.pages[]` (or the one named path); Resume names the pending page
Violation: STOP, rebuild the table from the catalog, then continue

CORRECT: emit writes the pages the catalog lists (billing: overview, components, contracts, pipelines); the Docs table has one `pending` row per listed DocPage plus each subprogram/actuator leaf before the first overview turn

PROHIBITED: filling `overview.md` in chat with no table row, then asking "next page?"

**CONSTRAINT:** One DocPage per turn. Stop at the first human gate.

- MUST: present exactly one `pending` (or `filled` awaiting revise-apply) page
- MUST: tutor four beats (situation, why, what the stub/catalog shows, then numbered choices) before fill, skip-none, or revise-apply
- MUST NOT: fill every kind for a slice in one message; MUST NOT dump the stub and ask "looks good?"

Enforcement: at most one Docs row Status changed this turn (except bulk Later the operator named)
Violation: STOP, revert extra page writes, finish the open page gate

CORRECT: this turn cold-reads and gates `docs/develop/billing/overview.md`; `components.md` stays `pending`

PROHIBITED: rewriting every billing page and program leaf, then asking the operator to confirm the cluster

**CONSTRAINT:** Catalog and repo are the only sources of surface.

- MUST: quote `.typology/typology.yaml` (or the page) in the evidence gate before FILL or SKIP-NONE
- MUST: SKIP-NONE when the kind has no matching components or opRuns (see [reference.md](reference.md) page-kind contracts)
- MUST NOT: invent endpoints, flags, viewer routes, or DSPy jobs the catalog does not name

Enforcement: every FILL or SKIP-NONE gate has a YES quote or an explicit empty-surface quote from the catalog table
Violation: STOP, delete invented surface, re-run the gate from the catalog

CORRECT: contracts page SKIP-NONE because `owns` has no `layer: interaction` `kind: api`; one sentence on the page

PROHIBITED: adding `/v1/invoice` because "billing probably has HTTP"

**CONSTRAINT:** Evaluation boxes run before the page body.

- MUST: complete cold-read phases STRUCTURE, INTENT, GAPS, CLASSIFY using the templates in [reference.md](reference.md); MUST NOT blend phases
- MUST: complete the evidence gate (Candidate, Catalog evidence, Decision FILL | SKIP-NONE | DEFER) before writing body
- MUST NOT: write fill prose until the gate Decision is FILL or SKIP-NONE; REVISE is post-fill only

Enforcement: outgoing message contains the four cold-read blocks and the gate before any page write
Violation: STOP, revert the page write, present cold-read and gate, wait

CORRECT: stub classifies EXPAND; gate Decision FILL with a quote of subprogram `objective:` from the catalog; then write leaf body

PROHIBITED: replacing the stub in the same turn as opening the file, with no gate

**CONSTRAINT:** After an accepted fill, the emit marker is gone. Emit MUST NOT clobber the page.

- MUST: remove `<!-- typology:generated -->` when the operator accepts the fill (or skip-none one-liner)
- MUST: leave the marker only while the file is still an emit stub
- MUST NOT: strip the marker to force emit; MUST NOT re-emit onto a page whose marker was removed

Enforcement: accepted page has no generated marker; `typology emit` skip rule in cli skill
Violation: STOP, restore from git if emit overwrote a filled page; re-remove the marker after the accepted fill

CORRECT: operator accepts overview fill; agent writes the body and deletes the marker

PROHIBITED: filled overview still starts with `<!-- typology:generated -->`, so the next emit replaces it

## Steps

1. **Resume**: if a journey file exists, Read it. If phase is not `docs` and the operator asked only to fill docs, load this skill anyway for the named pages; do not rediscover.
2. **Build the table**: one row per `docs.pages[]` on the confirmed catalog, plus one row per `subprograms/<id>.md` and `actuators/<id>.md`. Status `pending`.
3. **Pick one pending page**: tutor four beats. Load [reference.md](reference.md) page-kind contract for that kind.
4. **Cold-read**: four phases, four output blocks. Generated stubs usually CLASSIFY as EXPAND.
5. **Evidence gate**: Decision FILL, SKIP-NONE, or DEFER. Wait if DEFER (`later` on the row).
6. **Write**: FILL: body per the page-kind contract, then set Status `filled`. SKIP-NONE: one sentence that this slice has no that surface; set Status `skip-none`; remove the marker; that row is complete.
7. **Revise**: on `filled` rows, run the five-filter analysis in [reference.md](reference.md). Quote violations. Wait for apply. After apply, Status `revised` then `done` when filters pass. SKIP-NONE rows skip revise.
8. **Stop**: update Resume. MUST NOT start the next page in the same message. When no `pending` / `filled` / `revised` remain (`later` allowed), journey MAY set phase `done`.

## Page choices

End a fill turn with:

```markdown
1. **Accept fill**: write (or keep) this body; remove the generated marker.
2. **Skip-none**: this slice has no that surface; one-line page.
3. **Defer**: Status `later`; next pending page next turn.
4. **Revise first**: stay on this page; run five-filter before accept.
```

After Accept fill, next turn is five-filter (unless SKIP-NONE already completed the row).

## Pre-completion checklist

This checklist is per turn, not whole-docs done.

- [ ] **Table:** Docs table exists (journey file or one-off) and Resume names this page
      Method: Read table vs catalog `docs.pages[]`
      Pass: row present; Status matches the unit just closed or the gate waiting
      Fail: STOP, rebuild the table
- [ ] **One page:** at most one row Status changed
      Method: diff the journey file or chat table
      Pass: one page (or operator-named Later set)
      Fail: STOP, revert extra page writes
- [ ] **Boxes first:** cold-read four phases and evidence gate appear before any body write
      Method: reread the outgoing message vs git diff of the page
      Pass: boxes present; no body write on DEFER
      Fail: STOP, revert body, present boxes
- [ ] **Source:** FILL/SKIP-NONE quotes catalog or empty-surface evidence
      Method: gate Catalog evidence line
      Pass: YES with quote, or SKIP-NONE with empty owns/opRuns quote
      Fail: STOP, delete invented surface
- [ ] **Marker:** accepted fill or skip-none has no `<!-- typology:generated -->`
      Method: grep the page
      Pass: marker absent after accept; present only on still-stub pages
      Fail: STOP, remove the marker on accepted pages
