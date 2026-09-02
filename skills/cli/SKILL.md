---
name: typology-cli
description: >-
  Run the Typology CLI: discover a Go repo into architecture/typology.yaml,
  emit doc skeletons, validate paths and imports, show a slice, remediate one
  slice. Load when the user asks to discover, emit, validate, show, or remediate
  a typology catalog, or when ValidateStructure is not enough and repo files
  must be checked.
---

# Typology CLI

**Moral:** Discover drafts. A human confirms slice names. Emit writes docs only where the `<!-- typology:generated -->` marker is present. Validate fail-closes. Remediate scopes the agent to one slice.

**Binary:** `make build` → `./bin/typology` · **Catalog skill:** [catalog/SKILL.md](../catalog/SKILL.md) (load first when the YAML shape is in doubt) · **First map:** [journey/SKILL.md](../journey/SKILL.md) · **DocPages:** [docs/SKILL.md](../docs/SKILL.md)

## When to load

- First catalog on a Go module (load [journey/SKILL.md](../journey/SKILL.md) first; this skill is the command layer)
- Refreshing DocPage skeletons under `docs/develop`
- `typology validate` failures (missing path, import, or doc page)
- Scoping a fix to one slice (`remediate`)

## Commands

```text
typology discover REPO [--out PATH] [--docs-root PATH]
typology emit REPO [--catalog PATH] [--docs-only] [--go-only]
typology validate REPO [--catalog PATH] [SLICE]
typology show [SLICE] [--json] [--catalog PATH]
typology remediate REPO SLICE [--catalog PATH]
typology version
```

Default catalog: `REPO/architecture/typology.yaml`. `show` with no `--catalog` looks for that path or `typology.yaml` via `catalog.FindCatalog`.

## Steps

1. **Discover**: on a first map, follow [journey/SKILL.md](../journey/SKILL.md) (`--out architecture/typology.draft.yaml`). Otherwise `typology discover REPO` writes a draft catalog from the Go import graph. MUST NOT treat the draft as final slice names.
2. **Confirm**: a human accepts or renames slices and bindings (journey slice walk on a first map). MUST NOT emit or validate-as-done on unconfirmed names when this is a first map.
3. **Emit**: `typology emit REPO` writes catalog YAML (unless `--docs-only`), a slice README hub, DocPage skeletons (unless `--go-only`), and program leaves. MUST NOT overwrite a doc page that exists and lacks `<!-- typology:generated -->`. On a first map, load [docs/SKILL.md](../docs/SKILL.md) after emit (journey phase `docs`). Default `docs.pages` follows surfaces and opRuns; MUST NOT treat six missing files per slice as architecture failure.
4. **Fill catalog** — add subprograms, actuators, opRuns, bindings per [catalog/SKILL.md](../catalog/SKILL.md). Owned paths MUST exist as directories.
5. **Validate** — `typology validate REPO` (optional slice id). MUST fix every issue. MUST NOT ignore import or missing-path findings.
6. **Remediate** — `typology remediate REPO SLICE` when the job is one slice. MUST follow the returned `protocol`. MUST NOT refactor other slices in that pass.

## Core constraints

**CONSTRAINT:** Validate is fail-closed. A non-empty issue list means the catalog or repo is not done.

- MUST: run `typology validate REPO` after catalog or owned-path edits
- MUST NOT: claim success while validate prints issues

Enforcement: CLI exit non-zero when issues exist (`internal/cli` validate command)
Violation: STOP, fix the listed issues, re-run validate

CORRECT: `typology validate .` with empty issue output after a catalog change

PROHIBITED: editing `architecture/typology.yaml` and skipping validate

**CONSTRAINT:** Emit MUST NOT clobber human docs. Pages without `<!-- typology:generated -->` stay untouched.

- MUST: leave existing non-generated markdown in place
- MUST NOT: strip the marker to force a rewrite; MUST NOT hand-edit a generated page and leave the marker if you want emit to keep replacing it (remove the marker to take ownership)

Enforcement: `internal/emit/emit.go` skip when file exists and marker missing
Violation: STOP, restore the human page if emit overwrote it; keep the marker only on generated stubs

CORRECT: generated stub starts with `<!-- typology:generated -->`; human rewrite removes the marker

PROHIBITED: `typology emit` used to replace a finished human `overview.md` that still had the marker after substantial edits you meant to keep

**CONSTRAINT:** Remediate is one slice. The protocol from `remediate.Run` is mandatory for that pass.

- MUST: load that slice from the catalog; fix only packages that slice owns; re-validate
- MUST NOT: drive-by refactors on other slices in the same remediate pass

Enforcement: `typology remediate REPO SLICE` JSON/text `protocol` array; `validate` with `[SLICE]`
Violation: STOP, revert out-of-slice edits, finish the named slice

CORRECT: `typology remediate . billing` then only `internal/billing/...`

PROHIBITED: remediate `billing` then rewrite `internal/ledger` in the same pass

**CONSTRAINT:** Discover is a draft. Slice ids from package clusters are proposals.

- MUST: keep human-confirmed `architecture/typology.yaml` as source after the first confirm
- MUST NOT: re-discover over a confirmed catalog without an explicit replace request

Enforcement: operator intent; `discover --out` default is `architecture/typology.yaml` (overwrites if you pass the same path)
Violation: STOP, restore the confirmed catalog from git, discover to a temp `--out` if you need a diff

CORRECT: `typology discover . --out /tmp/draft-typology.yaml` then merge by hand

PROHIBITED: `typology discover .` onto a confirmed catalog with no backup

## Library equivalent

When the consumer already has a `catalog.Typology` in memory (no CLI):

```go
issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: t})
rep, err := remediate.Run(remediate.Options{RepoRoot: repo, Catalog: t, SliceID: "billing"})
```

`validate.Run` with empty `RepoRoot` is structure-only. Path and import checks need the repo.

## Pre-completion checklist

- [ ] **Catalog skill applied:** YAML matches [catalog/SKILL.md](../catalog/SKILL.md)
      Method: structure validate clean
      Pass: no structure issues
      Fail: STOP, fix catalog, then CLI validate
- [ ] **Validate clean:** `typology validate REPO` (or scoped slice) reports no issues
      Method: CLI output empty of issue lines; exit 0
      Pass: exit 0
      Fail: STOP, fix each issue
- [ ] **Docs:** every `docs.pages[].path` exists after emit or hand authoring; every subprogram and actuator has its leaf page
      Method: validate DocPage and program-page findings
      Pass: none
      Fail: STOP, emit or create the file
- [ ] **Remediate scope:** if `remediate` ran, diff is limited to that slice's owned paths
      Method: git diff paths vs slice `owns`
      Pass: no extra packages
      Fail: STOP, revert other slices
