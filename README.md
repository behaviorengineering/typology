# Typology

Portable Go library and CLI to **discover** bounded contexts, **write** the map as code, **validate** paths and imports, and **scope** AI debt fixes to one slice at a time.

**Module:** [`github.com/behaviorengineering/typology`](https://github.com/behaviorengineering/typology)

Consilium is a consumer via `engine/internal/typologyadapter` and `consilium-pii typology export`. It does **not** replace `slice validate`.

## Install

Download a release binary for your OS from [GitHub Releases](https://github.com/behaviorengineering/typology/releases), or build from source:

```bash
make build
./bin/typology version
```

In Consilium (submodule):

```bash
make -C providers/typology build
providers/typology/bin/typology version
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
3. `typology emit` writes catalog YAML and develop DocPage skeletons.
4. `typology validate` fails closed on missing paths, bindings, or docs.
5. `typology remediate REPO SLICE` returns agent-scoped violations.

## Consilium dogfood

```bash
bin/consilium-pii typology export --out /tmp/consilium-typology.yaml
```

## Layout

```text
providers/typology/
  cmd/typology/           CLI entry
  catalog/              Typology model + YAML I/O
  validate/             Path + import + DocPage checks
  internal/discover/    Go import graph → draft catalog
  internal/emit/        YAML + DocPage markdown
  internal/remediate/   Agent protocol for one slice
  internal/cli/         Command dispatch
  testdata/tiny-module/ Fixture Go module
```

## Consilium

```bash
bin/consilium-pii typology export     # architecture/typology.yaml
bin/consilium-pii typology validate   # fail if stale or paths/docs missing
make typology-validate                # wired into validate-viewer-sections
```

Adapter: `engine/internal/typologyadapter` maps `Contract()` → Slice owns (RuleGlobs/Tests), Jobs (CLI actions), DocCluster (`Docs`).

| Type | Role |
|------|------|
| `Typology` | Whole map |
| `Slice` | Bounded context |
| `Component` | Package (`domain` or `interaction` ui/cli/api) |
| `Job` | Background work surfaced on pipelines page |
| `SliceBinding` | Coupling between slices |
| `ComponentBinding` | Coupling between components |
| `DocCluster` / `DocPage` | Develop doc set per slice |

JSON/CALM export is future work; humans edit YAML/Go.
