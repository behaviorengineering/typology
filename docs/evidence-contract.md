# Package evidence contract

Language-neutral artifacts written by Typology harvest and consumed by Majordomo
gates. Harvest is per language; file names and YAML fields stay stable.

## Files

| File | Role |
|---|---|
| `tmp/typology/package_roles.yaml` | Observed roles and labeled import edges |
| `tmp/typology/package_contracts.md` | Public-contract summary for LLMs |
| `tmp/typology/package_rlm_context.md` | Progressive AST context (bodies fenced per language) |

Majordomo copies these under `evidence/typology/`.

## `package_roles.yaml`

```yaml
packages:
  - path: internal/board          # required; repo-relative package path
    role: dto                     # entrypoint|server|dto|exec_runner|aggregator|adapter|config|observability|unknown
    confidence: 0.90
    evidence: [json_tags, ...]    # portable token strings; never folder names
    inspected_stage: 1
    language: go                  # go|python (optional for readers; writers always set)
    candidate_role: ...           # optional
edges:
  - from: internal/server
    to: internal/board
    kind: fills_dto               # importer fills a dto target
```

Edge kinds: `fills_dto`, `uses_runner`, `serves_server`, `composes`, `reads_config`, `imports`.

## Portable delivery tokens

Harvests map language-specific AST signals onto shared evidence tokens and
`PackageEvidence` flags. Tokens that appear in role `evidence` lists:

| Token | Meaning |
|---|---|
| `has_main` / `cli_main` | Entrypoint / `__main__` / console script |
| `json_tags` / `typed_fields` | Serialization or typed data shapes (Go tags; Pydantic/dataclass/TypedDict) |
| `delivery:http` / `http_surface` | HTTP app or handler surface |
| `delivery:grpc` | gRPC service surface |
| `imports_net_http` / `http_surface_ident` | HTTP client or server imports |
| `imports_os_exec` / `subprocess_exec` | Process execution |
| `imports_otel` / `imports_prometheus` | Observability boot |
| `go_embed` / `embeds_static` | Embedded static UI (Go) |
| `orchestration_export` | Aggregator/collect/build surface |
| `load_save_exports` | Config load/save surface |
| `exports_client_surface` | Thin external adapter |

## Contracts / RLM markdown

Heading and flag style stay stable (`hasMain`, delivery flags, role, confidence).
Body fences use `go` or `python` from the package `language`. Majordomo gates do
not parse fences.

## Harvest languages

| Language | Root markers | Parser |
|---|---|---|
| Go | `go.mod` / `go.work` | `go/parser` + `go list` |
| Python | `pyproject.toml`, `setup.cfg`, `setup.py` | Pure-Go [gopapy](https://github.com/tamnd/gopapy) (no CPython) |

Both harvests feed the same role classifier and writers. Mixed repos merge
packages into one topology.
