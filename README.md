# Typology

Portable Go library and CLI to **discover** bounded contexts, **write** the map as code, **explain** the design for human review, **validate** paths and imports, and **scope** AI debt fixes to one slice at a time.

**Module:** [`github.com/behaviorengineering/typology`](https://github.com/behaviorengineering/typology)

Agents: start at [AGENTS.md](AGENTS.md). Skills live in [ai-copilots/skills/](ai-copilots/skills/) (any agent; wire with [ai-copilots/BOOTSTRAP.md](ai-copilots/BOOTSTRAP.md)).

## Install

**Prebuilt binary:** download for your OS from [GitHub Releases](https://github.com/behaviorengineering/typology/releases).

**Go install (PATH):**

```bash
go install github.com/behaviorengineering/typology/cmd/typology@latest
typology version
```

**Build from source** (this checkout):

```bash
make build
./bin/typology version
```

**Consumer Go tool** (pins the CLI in a consuming module's `go.mod`):

```text
go run github.com/behaviorengineering/typology/cmd/typology@latest init .
go tool typology version
```

The bootstrap command updates the selected consumer `go.mod` and `go.sum`. It does not add the CLI to application imports or binaries. For a multi-module workspace, pass the module path explicitly:

```text
go run github.com/behaviorengineering/typology/cmd/typology@latest init . --module engine
```

To use the catalog package in application code, pin the library separately:

```text
go get github.com/behaviorengineering/typology@latest
go mod tidy
```

Prefer a concrete `vX.Y.Z` tag in CI and published modules once you settle on a release.

## Commands

| Command | Role |
|---------|------|
| `typology init REPO` | Register the CLI as a Go tool in the consumer module |
| `typology discover REPO` | Draft catalog under `tmp/typology/` |
| `typology emit REPO` | Write confirmed catalog, agent README, DocPage skeletons |
| `typology architecture REPO` | Human architecture brief under `docs/architecture/` |
| `typology assembly-graph REPO` | Machine cable board JSON (optional `--slice` / `--all-slices`) |
| `typology boards register` | Interactive TTY wizard, or `REPO BOARD_ID` / `--all-slices` for CI |
| `typology boards sync` | Rematerialize Vite `public/` from the XDG registry |
| `typology boards path` | Print YAML, config, or data directory path |
| `typology validate REPO` | Fail-closed path, import, and DocPage checks |
| `typology show` | Slice or import-graph summary |
| `typology remediate REPO SLICE` | Agent-scoped violations for one slice |
| `typology version` | Print version |

```text
typology init REPO [--module PATH] [--version VERSION]
typology discover REPO [--module PATH] [--out PATH] [--docs-root PATH]
typology emit REPO [--catalog PATH] [--docs-only] [--go-only]
typology architecture REPO [--module PATH] [--catalog PATH] [--out PATH]
typology assembly-graph REPO [--module PATH] [--catalog PATH] [--slice SLICE|--all-slices] [--out PATH|--out-dir DIR]
typology boards register REPO BOARD_ID [--prefix PREFIX] [--slice ID] [--viewer PUBLIC_DIR] [...]
typology boards register REPO --all-slices [--prefix PREFIX] [--viewer PUBLIC_DIR] [...]
typology boards register   # interactive wizard (TTY)
typology boards sync --viewer PUBLIC_DIR
typology boards path [--yaml|--config|--data]
typology validate REPO [--module PATH] [--catalog PATH] [SLICE]
typology show [SLICE|graph] [--module PATH] [--json] [--catalog PATH]
typology remediate REPO SLICE [--module PATH] [--catalog PATH]
typology version
```

## Consumer setup

Typology is guidance for agents that design and structure code. Any Go library that adopts it gets the same layout when you run `typology emit` in that repo.

One repository owns one Typology catalog and its architecture documentation. In a multi-module repository, `scope.modules` declares the repository-local modules covered by the catalog; `go.work` does not widen that scope.

| Path | Role |
|------|------|
| `.typology/typology.yaml` | Confirmed catalog and module scope (source of truth) |
| `.typology/README.md` | Agent instructions: skills, commands, catalog-first workflow |
| `.typology/tools.yaml` | Generated CLI tool index from `opRuns` |
| `.typology/typology-journey.md` | First-map session file (journey skill) |
| `tmp/typology/typology.yaml` | Discover draft (not the confirmed catalog) |
| `tmp/typology/assembly-graph.json` | Cable board JSON (CLI: `assembly-graph`; agent skill: `typology-cable-board`) |
| `AGENTS.md` | Pointer to `.typology/README.md` (created or appended by emit) |
| `docs/develop/` | Per-slice DocPages (default docs root) |
| `docs/architecture/typology.md` | Human-readable catalog and Go-topology comparison |

Operator board registry (not in the consumer repo; lives on the machine that runs `boards register`):

| Path | Role |
|------|------|
| `~/.config/typology/boards.yaml` | Durable board registry (`TYPOLOGY_CONFIG_DIR` override) |
| `~/.local/share/typology/boards/<id>/` | Harvested graphs (`TYPOLOGY_DATA_DIR` override) |

Day-to-day in a consumer: update the catalog first, implement code to match it, then `typology validate`. Agents load the skills listed in `.typology/README.md` before changing architecture or package layout.

## Workflow

First map in a new repo: load [ai-copilots/skills/journey/SKILL.md](ai-copilots/skills/journey/SKILL.md) (plan file `.typology/typology-journey.md`, discover to a draft, walk slices, emit, then fill DocPages with [ai-copilots/skills/docs/SKILL.md](ai-copilots/skills/docs/SKILL.md)).

1. `typology discover` on a Go repo (draft; writes to `tmp/typology/typology.yaml` by default). Use `--module` when a multi-module workspace has no catalog scope yet.
2. Human confirms slice names and bindings.
3. `typology emit` writes `.typology/typology.yaml`, `.typology/README.md`, `.typology/tools.yaml`, ensures `AGENTS.md` points at `.typology/README.md`, plus DocPage skeletons under the docs root (default `docs/develop`). Empty CLI/UI/API/Jobs pages are omitted unless listed in YAML.
4. `typology architecture` writes a deterministic Markdown projection under `docs/architecture/typology.md`. It combines the intended catalog with observed package topology within `scope.modules` and names findings for human review. It does not make narrative design decisions.
5. Cable board:
   - Machine checks: `typology assembly-graph REPO` writes JSON (`tmp/typology/assembly-graph.json` by default). With `--slice` or `--all-slices`, each board is one bounded context plus boundary stubs.
   - Human viewer: from inside the consumer repo run `typology boards register` (TTY wizard), or for CI `typology boards register REPO BOARD_ID --viewer PUBLIC_DIR` / `--all-slices --prefix PREFIX`. That harvests into XDG, upserts `boards.yaml`, and materializes [`viewer/cable-board/`](viewer/cable-board/). Multi-repo boards MUST use `--prefix` so ids and localStorage keys stay unique.
   - Agents load [ai-copilots/skills/cable-board/SKILL.md](ai-copilots/skills/cable-board/SKILL.md).
6. An agent or architect fixes each finding or records the boundary debt in the journey file.
7. `typology validate` fails closed on missing paths, bindings, DocPages, or program leaves.
8. `typology remediate REPO SLICE` returns agent-scoped violations for one slice.

## Layout

```text
AGENTS.md             Pointer for coding agents
ai-copilots/          Portable agent pack (BOOTSTRAP + skills: journey, docs, catalog, CLI, cable-board)
catalog/              Typology model + YAML I/O
architecture/         Human-readable catalog and topology reports
assemblygraph/        Cable board JSON (portable package wiring)
boardregistry/        Durable XDG YAML registry + share-dir graphs + viewer materialize
viewer/cable-board/   Interactive cable board UI (materialized public/ projection)
validate/             Path + import + DocPage checks
cmd/typology/         CLI entry
internal/discover/    Go import graph → draft catalog
internal/emit/        YAML + DocPage markdown
internal/remediate/   Agent protocol for one slice
internal/cli/         Command dispatch (includes boards wizard)
testdata/tiny-module/ Fixture Go module
```

## Model

| Type | Role |
|------|------|
| `Typology` | Whole map |
| `Slice` | Bounded context with a required business `objective` |
| `Component` | Package. Domain on `owns[]`; interaction nested under a `Surface`. |
| `Surface` | Built UI, CLI, or API artefact (`kind` + `components[]`). |
| `OpRun` | One gated operator invocation (CLI, HTTP, human, signal, or schedule). Optional `runs` / `actuates`. |
| `Subprogram` | Standing program: required `objective`, plus `input`, `output`, optional `store`, `gate` |
| `Actuator` | Signal-triggered capability that emits an effect, usually past the edge |
| `SliceBinding` | Coupling between slices |
| `ComponentBinding` | Coupling between components |
| `DocCluster` / `DocPage` | Doc set per slice. Kinds are leaves. Nav is Overview → Owns → Subprograms → Surfaces (CLI, UI, API, Jobs). |

JSON/CALM export is future work; humans edit YAML/Go.

## CI

Pull requests and pushes to `main` run `go vet ./...` and `go test ./...` (workflow `ci.yml`). Local smoke: `make smoke` (includes assembly-graph contract checks and prefixed board register).

## Releases

Typology ships a CLI binary plus a Go library. Releases are `v*` tags plus a GitHub Release from GoReleaser ([`.goreleaser.yaml`](.goreleaser.yaml)).

**Auto patch on `main`:** every push to `main` that is not docs/chore/ci-only creates `vX.Y.(Z+1)` and publishes a release (workflow `auto-patch-release.yml`). Put `[skip release]` in the commit subject to opt out once.

**Skip (no tag):** when every commit subject since the last `v*` tag is only `docs:`, `chore:`, or `ci:` (conventional prefixes).

**Manual minor/major:** run workflow **Auto patch release** with `bump=minor` or `bump=major` (or push a `v*` tag yourself). Use major only for breaking public API changes.

After a release lands, consumers bump with `go get github.com/behaviorengineering/typology@vX.Y.Z` (or `@latest`) and re-run `go install` / `init` if they want the new CLI binary.
