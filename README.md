# Typology

Portable Go library and CLI to **discover** bounded contexts, **write** the map as code, **validate** paths and imports, and **scope** AI debt fixes to one slice at a time.

**Module:** [`github.com/behaviorengineering/typology`](https://github.com/behaviorengineering/typology)

## Install

Download a release binary for your OS from [GitHub Releases](https://github.com/behaviorengineering/typology/releases), or build from source:

```bash
make build
./bin/typology version
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

1. `typology discover` on a Go repo (draft `architecture/typology.yaml`).
2. Human confirms slice names and bindings.
3. `typology emit` writes catalog YAML and DocPage skeletons under the docs root (default `docs/develop`).
4. `typology validate` fails closed on missing paths, bindings, or docs.
5. `typology remediate REPO SLICE` returns agent-scoped violations for one slice.

## Layout

```text
cmd/typology/         CLI entry
catalog/              Typology model + YAML I/O
validate/             Path + import + DocPage checks
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
| `Component` | Package (`domain` or `interaction` ui/cli/api) |
| `Job` | Background work surfaced on a pipelines page |
| `SliceBinding` | Coupling between slices |
| `ComponentBinding` | Coupling between components |
| `DocCluster` / `DocPage` | Doc set per slice |

JSON/CALM export is future work; humans edit YAML/Go.

## Releases (for agents)

Typology ships a CLI binary plus a Go library. Releases are `v*` tags plus a GitHub Release from GoReleaser.

**Auto patch on `main`:** every push to `main` that is not docs/chore/ci-only creates `vX.Y.(Z+1)` and publishes a release (workflow `auto-patch-release.yml`). Put `[skip release]` in the commit subject to opt out once.

**Skip (no tag):** when every commit subject since the last `v*` tag is only `docs:`, `chore:`, or `ci:` (conventional prefixes).

**Manual minor/major:** run workflow **Auto patch release** with `bump=minor` or `bump=major` (or push a `v*` tag yourself). Use major only for breaking public API changes.

**After a new tag, consumer agents MUST pin:**

```bash
git -C providers/typology fetch --tags origin
git -C providers/typology checkout "vX.Y.Z"
go get github.com/behaviorengineering/typology@vX.Y.Z
go mod tidy
```
