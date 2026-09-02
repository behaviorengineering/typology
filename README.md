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

Pin the module in a consumer:

```bash
go get github.com/behaviorengineering/typology@vX.Y.Z
go mod tidy
```

## Commands

```text
typology discover REPO [--out PATH] [--docs-root PATH]
typology emit REPO [--catalog PATH] [--docs-only] [--go-only]
typology validate REPO [--catalog PATH] [SLICE]
typology show [SLICE] [--json] [--catalog PATH]
typology remediate REPO SLICE [--catalog PATH]
typology version
```

## Workflow

First map in a new repo: load [skills/journey/SKILL.md](skills/journey/SKILL.md) (plan file `architecture/typology-journey.md`, discover to a draft, walk slices, emit, then fill DocPages with [skills/docs/SKILL.md](skills/docs/SKILL.md)).

1. `typology discover` on a Go repo (draft; first map uses `--out architecture/typology.draft.yaml`).
2. Human confirms slice names and bindings.
3. `typology emit` writes catalog YAML, a slice README hub (tree nav), DocPage skeletons, and `subprograms/` / `actuators/` leaves under the docs root (default `docs/develop`). Empty CLI/UI/API/Jobs pages are omitted unless listed in YAML.
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
| `Slice` | Bounded context |
| `Component` | Package. Domain on `owns[]`; interaction nested under a `Surface`. |
| `Surface` | Built UI, CLI, or API artefact (`kind` + `components[]`). |
| `OpRun` | One gated operator invocation (CLI, HTTP, human, signal, or schedule). Optional `runs` / `actuates`. |
| `Subprogram` | Standing program: `input`, `output`, optional `store`, `gate` |
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
