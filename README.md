# Typology

Portable Go library and CLI to **discover** bounded contexts, **write** the map as code, **validate** paths and imports, and **scope** AI debt fixes to one slice at a time.

**Module:** [`github.com/behaviorengineering/typology`](https://github.com/behaviorengineering/typology)

Agents: start at [AGENTS.md](AGENTS.md). Skills live in [skills/](skills/) (any agent; optional symlink into your host skill folder).

## Install

Download a release binary for your OS from [GitHub Releases](https://github.com/behaviorengineering/typology/releases), or build from source:

```bash
make build
./bin/typology version
```

In a consuming Go module, register the CLI as a Go tool:

```text
go run github.com/behaviorengineering/typology/cmd/typology@v0.0.5 init .
go tool typology version
```

The bootstrap command updates the selected consumer `go.mod` and `go.sum`. It does not add the CLI to application imports or binaries. For a multi-module workspace, pass the module path explicitly:

```text
go run github.com/behaviorengineering/typology/cmd/typology@v0.0.5 init . --module engine
```

To use the catalog package in application code, pin the library separately:

```text
go get github.com/behaviorengineering/typology@v0.0.5
go mod tidy
```

## Commands

```text
typology init REPO [--module PATH] [--version VERSION]
typology discover REPO [--out PATH] [--docs-root PATH]
typology emit REPO [--catalog PATH] [--docs-only] [--go-only]
typology validate REPO [--catalog PATH] [SLICE]
typology show [SLICE] [--json] [--catalog PATH]
typology remediate REPO SLICE [--catalog PATH]
typology version
```

## Consumer setup

Typology is guidance for agents that design and structure code. Any Go library that adopts it gets the same layout when you run `typology emit` in that repo.

| Path | Role |
|------|------|
| `.typology/typology.yaml` | Confirmed catalog (source of truth) |
| `.typology/README.md` | Agent instructions: skills, commands, catalog-first workflow |
| `.typology/tools.yaml` | Generated CLI tool index from `opRuns` |
| `.typology/typology-journey.md` | First-map session file (journey skill) |
| `tmp/typology/typology.yaml` | Discover draft (not the confirmed catalog) |
| `AGENTS.md` | Pointer to `.typology/README.md` (created or appended by emit) |
| `docs/develop/` | Per-slice DocPages (default docs root) |

Day-to-day in a consumer: update the catalog first, implement code to match it, then `typology validate`. Agents load the skills listed in `.typology/README.md` before changing architecture or package layout.

## Workflow

First map in a new repo: load [skills/journey/SKILL.md](skills/journey/SKILL.md) (plan file `.typology/typology-journey.md`, discover to a draft, walk slices, emit, then fill DocPages with [skills/docs/SKILL.md](skills/docs/SKILL.md)).

1. `typology discover` on a Go repo (draft; writes to `tmp/typology/typology.yaml` by default).
2. Human confirms slice names and bindings.
3. `typology emit` writes `.typology/typology.yaml`, `.typology/README.md`, `.typology/tools.yaml`, ensures `AGENTS.md` points at `.typology/README.md`, plus DocPage skeletons under the docs root (default `docs/develop`). Empty CLI/UI/API/Jobs pages are omitted unless listed in YAML.
4. `typology validate` fails closed on missing paths, bindings, DocPages, or program leaves.
5. `typology remediate REPO SLICE` returns agent-scoped violations for one slice.

## Layout

```text
AGENTS.md             Pointer for coding agents
skills/               Portable agent skills (journey, docs, catalog, CLI)
catalog/              Typology model + YAML I/O
validate/             Path + import + DocPage checks
cmd/typology/         CLI entry
internal/discover/    Go import graph → draft catalog
internal/emit/        YAML + DocPage markdown
internal/remediate/   Agent protocol for one slice
internal/cli/         Command dispatch
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

Pull requests and pushes to `main` run `go vet ./...` and `go test ./...` (workflow `ci.yml`).

## Releases

Typology ships a CLI binary plus a Go library. Releases are `v*` tags plus a GitHub Release from GoReleaser.

**Auto patch on `main`:** every push to `main` that is not docs/chore/ci-only creates `vX.Y.(Z+1)` and publishes a release (workflow `auto-patch-release.yml`). Put `[skip release]` in the commit subject to opt out once.

**Skip (no tag):** when every commit subject since the last `v*` tag is only `docs:`, `chore:`, or `ci:` (conventional prefixes).

**Manual minor/major:** run workflow **Auto patch release** with `bump=minor` or `bump=major` (or push a `v*` tag yourself). Use major only for breaking public API changes.
