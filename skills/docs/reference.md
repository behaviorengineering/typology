# Typology docs reference

LOAD-WHEN: filling or evaluating a Typology DocPage (`overview`, `components`, `contracts`, `cli`, `presentation`, `pipelines`). The skill [SKILL.md](SKILL.md) owns when to load and the turn cadence. This file is the evaluation pack.

MUST NOT use Consilium `revise-flow` checklists here. MUST NOT require “We [verb]”. MUST NOT use H2 emoji or teacher TOCs (“What You’ll Learn”). MUST NOT use U+2014.

Voice: active, named actor (slice, operator, package). Gloss jargon in the same breath on first use (`slice` = bounded context, `owns` = packages, `binding` = coupling).

---

## Cold-read (four phases)

MUST complete in order. MUST NOT blend phases. Each phase MUST fill the template before the next.

Generated emit stubs (marker present, tables only) usually CLASSIFY as EXPAND.

### STRUCTURE

```
STRUCTURE:
- Title: [exact H1]
- Path: [docs/develop/<slice>/<kind>.md]
- Kind: overview | components | contracts | cli | presentation | pipelines
- Marker: generated | human
- Headings: [H1 > H2 …]
- Tables: [count and names]
- TOC: present | absent
```

**Rules:** MUST capture H1 and H2. MUST note the generated marker if present. MUST NOT summarise body prose in this phase.

### INTENT

```
INTENT:
- Audience: operator | agent | both
- Purpose: [one phrase, e.g. "slice overview", "API surface"]
- Tone: stub | reference | explanatory
- Opening states purpose: YES | NO
- Scope in: [what this kind covers]
- Scope out: [what other kinds cover]
```

**Rules:** MUST infer audience from the kind contract below, not from guesswork. MUST distinguish stub vs filled page.

### GAPS

```
GAPS:
- Critical missing: [list, or none]
- Under-explained: [jargon without gloss; tables without why]
- Structural: [flow vs kind contract]
- Catalog drift: [page disagrees with typology.yaml, or none]
- Reader journey: YES | NO
```

**Rules:** MUST evaluate gaps against the kind contract. MUST NOT fix gaps in this phase.

### CLASSIFY

Ask in order. Stop at first YES.

```
Q1 structural or intent error (wrong kind shape, broken headings)? YES → RESTRUCTURE
Q2 missing content that blocks the kind contract? YES → EXPAND
Q3 tone, missing gloss, or thin rationale? YES → REFINE
Q4 complete, only polish? YES → POLISH
else COMPLETE
```

```
CLASSIFY:
- Class: RESTRUCTURE | EXPAND | REFINE | POLISH | COMPLETE
- Why: [which question]
- Work: [3 concrete tasks, or none]
```

**Rules:** MUST stop at first YES. Work items MUST be concrete (e.g. "write why billing reads ledger").

---

## Evidence gate

Fill this block in the message BEFORE writing page body. YES MUST quote evidence from `architecture/typology.yaml` or the current page. MUST NOT self-report YES.

Gate Decision has three outcomes only. REVISE is a later turn after FILL.

```
Candidate: [path]
Kind: [kind]
Catalog evidence: YES: [quote] / NO
Empty surface (this kind has no matching owns or opRuns): YES: [quote] / NO
Decision: FILL | SKIP-NONE | DEFER
If FILL: (1) first sentence: [clause]
         (2) catalog facts to keep: [tables or ids]
If SKIP-NONE: one-line page: [sentence]
If DEFER: later reason: [clause]
```

CORRECT:

```
Candidate: docs/develop/billing/overview.md
Kind: overview
Catalog evidence: YES: "objective: mint invoices from store"
Empty surface: NO
Decision: FILL
If FILL: (1) first sentence: Billing mints invoices; operators run mint-invoice.
         (2) catalog facts to keep: owns table store, httpapi, cli
```

PROHIBITED:

```
Candidate: overview.md
Kind: overview
Catalog evidence: YES
Decision: FILL
```

Violation: YES with no quote. MUST NOT write body.

Opening-states-purpose is checked **after** fill (five-filter step 1), not in this gate.

---

## Page-kind contracts

Evaluator checks the filled page against `architecture/typology.yaml` and `catalog.DocPageKind`. MUST NOT use training as the source of endpoints or commands.

### overview

MUST:

- MUST: First sentence states the bounded context and who operates it
- MUST: Point at the tree hub (`README.md`) and Owns (`components.md`); do not duplicate the owns inventory
- Prose explains the slice as a context, not a package dump

MUST NOT: list every file in `owns` as a bullet of “features”; omit the operator

CORRECT first sentence: `Billing mints invoices from store records. Operators run mint-invoice on the billing CLI.`

PROHIBITED first sentence: `This document provides an overview of the billing slice.`

### components

MUST:

- Keep Owns and Surfaces tables (those are reference lists)
- Keep Subprograms and Actuators index tables with links to `subprograms/<id>.md` and `actuators/<id>.md` when the catalog lists them
- State why each slice or component binding exists (rationale, one clause per binding) under Cross-slice
- Gloss subprogram (standing program), actuator (signal in, emit out), and component (package) on first use

MUST NOT: convert the owns table into a 5+ bullet “characteristics” list; drop a binding that is in the catalog

CORRECT: table of `reads` to ledger, then `Billing reads ledger balances so mint does not own the ledger store.`

PROHIBITED: deleting the SliceBindings table and writing “Billing talks to other slices.”

### contracts

MUST: document HTTP/API from `surfaces[]` with `kind: api` and their nested components. Read those packages when the catalog names them.

MUST NOT: invent paths or verbs. If no api surface: SKIP-NONE, one sentence.

CORRECT skip-none: `Billing has no api surface in the catalog.`

PROHIBITED: `/v1/invoices` with no api component and no code route.

### cli

MUST: operator surface from `surfaces[]` with `kind: cli` and opRuns with a `cli:` field.

MUST NOT: invent flags. If none: SKIP-NONE, one sentence.

CORRECT: `mint-invoice` from opRun `cli:` plus the owner component path.

PROHIBITED: a “common flags” section copied from another product.

### presentation

MUST: UI or viewer packages from `surfaces[]` with `kind: ui`.

MUST NOT: describe HTTP or CLI here. If none: SKIP-NONE, one sentence.

CORRECT skip-none: `Billing has no ui surface in the catalog.`

PROHIBITED: documenting the REST api on this page because “the UI will call it.”

### pipelines

MUST: one prose clause per opRun (what it `runs` or `actuates`, and `gate`). Keep the emit table.

MUST NOT: invent DSPy job names the catalog does not name. Empty opRuns table: SKIP-NONE, one sentence.

CORRECT: `OpRun mint-invoice runs subprogram invoice with gate auto.`

PROHIBITED: `chronology_select` on a slice whose opRuns list does not name it.

---

## Five-filter revise

Run after Status `filled`, before `done`. Analysis quotes exact phrases from the page. MUST NOT write until the operator says apply (or names filters). SKIP-NONE pages skip this pack.

Pass: no banned pattern remains for that filter. Fix Needed: quote the violation.

### 1 Opening clarity

- First sentence states purpose and operator (or skip-none already did)
- Title is the kind name or a specific slice+kind title, not a bland “Introduction”
- No mystery box (“more below”)

### 2 Voice scrub

Banned: teacher framing (“You will learn”), hedges (“it is important to note”), passive piles (“it was determined”), AI rhetoric (“this changes how we understand”), mandatory “We”

Fix: named actor + verb. Keep decision rationale.

### 3 Accessibility

- Technical terms from the catalog kept exactly; gloss in parentheses on first use
- MUST NOT drop `subprogram`, `actuator`, `opRun`, or slice ids
- Short paragraphs (1–3 sentences)

### 4 Formatting

- Zero U+2014
- Bold 2–5 word key phrases, not whole sentences; restrained (about 2–5 spans per section)
- Backticks for paths and ids, not for ordinary emphasis
- No H2 emoji

### 5 Structural integrity

- No negation-first (“this is not X, it is Y”)
- No dead-end closing that restates the page
- Examples (if any) sit next to the claim they illustrate
- Page still matches the kind contract (no contracts content on overview)

### Analysis output

For each filter:

```
### Filter N: [name]
Violations: [quoted phrases, or none]
| # | Location | Before | After |
Status: Pass | Fix Needed
```

Then ask: apply all filters, named filters only, or no changes.

---

## skip-none one-liner

CORRECT:

```markdown
# CLI

Billing has no interaction-layer cli component and no opRun with a cli field in the catalog.
```

No generated marker after accept. No invented flags.

PROHIBITED: a stub table of `_(none)_` left with the generated marker after the operator accepted skip-none.
