# Typology cable board viewer

Interactive ComfyUI-style board for the portable cable board JSON produced by
`typology assembly-graph` / `typology boards register`.

This viewer is the human surface for the [cable-board skill](../../ai-copilots/skills/cable-board/SKILL.md).
It is product-neutral: any consumer repo can harvest a graph and open this UI.

## Run

**Operators (no npm):** after `typology boards register`, run:

```bash
typology boards serve
# open http://127.0.0.1:5173/?board=<id>
```

The Go binary embeds the production SPA and always serves it in the
**foreground** (your terminal stays attached until Ctrl+C). Board JSON is read from
`~/.local/share/typology/viewer/public` (or `--viewer PUBLIC_DIR`).

**Developers editing this React app** (HMR + live XDG boards):

```bash
# from Typology module root (or a host that vendors it under providers/typology)
typology boards serve --dev
# or: make cable-board-dev
```

`--dev` starts Vite against the same XDG `boards.json` operators use. Pass
`--viewer-src` / `TYPOLOGY_VIEWER_SRC` if the cable-board sources are not found
from the current directory.

Slice boards also expose **Scope** controls: Internal, External dependents, and
Depends on others.

Release builds run `make cable-board-dist` so `internal/boardsviewer/dist` is
embedded for `go install` / GitHub Releases.

## Boards

Two layers of storage:

1. **Durable (operator machine):** YAML registry + harvested graphs under XDG.
2. **Projection (Vite `public/`):** generated on register/sync so the React app can fetch JSON. Only the tiny-module sample is committed.

| Path | Role |
|------|------|
| `~/.config/typology/boards.yaml` | Durable registry (`TYPOLOGY_CONFIG_DIR` override) |
| `~/.local/share/typology/boards/<id>/assembly-graph.json` | Harvested graphs (`TYPOLOGY_DATA_DIR` override) |
| `~/.local/share/typology/viewer/public/` | Wizard default materialize target (`boards.json` + graphs) |
| `public/boards.json` | Materialized manifest for the React app (checkout or XDG) |
| `public/boards/<id>/assembly-graph.json` | Materialized graphs Vite serves |

```bash
# Interactive wizard (TTY): run from inside a consumer repo
typology boards register

# Non-interactive (CI): harvest, upsert YAML, materialize
typology boards register /path/to/consumer-repo engine \
  --module engine --label "Engine module" \
  --viewer /path/to/typology/viewer/cable-board/public

# Rematerialize from the existing YAML registry (no harvest)
typology boards sync --viewer /path/to/typology/viewer/cable-board/public

typology boards path --yaml
```

Bare `boards register` without a TTY fails closed and prints usage (CI-safe).

Then open `http://localhost:5173/?board=engine` in one window and
`http://localhost:5173/?board=tiny-module` in another. The header switcher
moves between boards and keeps the board in the URL. Override any board with
`?src=/other.json`.

## Multi-repo boards

When several consumer repos share one Vite server, register each with an
explicit `--prefix` so board ids and localStorage keys never collide. The same
slice name in two repos becomes two boards in `boards.yaml`:

```bash
# Product A slices -> product-a-billing, … under ~/.config/typology/boards.yaml
typology boards register /path/to/product-a --all-slices \
  --prefix product-a --module engine \
  --catalog /path/to/product-a/.typology/typology.yaml \
  --viewer /path/to/typology/viewer/cable-board/public

# Product B with the same slice names -> product-b-billing, …
typology boards register /path/to/product-b --all-slices \
  --prefix product-b \
  --catalog /path/to/product-b/.typology/typology.yaml \
  --viewer /path/to/typology/viewer/cable-board/public

# One slice with prefix (idempotent if BOARD_ID already starts with prefix-):
typology boards register /path/to/product-a chronology \
  --slice chronology --prefix product-a --module engine \
  --catalog /path/to/product-a/.typology/typology.yaml \
  --viewer /path/to/typology/viewer/cable-board/public
```

Open `http://localhost:5173/?board=product-a-chronology` or filter with
`?repo=product-a`. Layout state is keyed by the full board id
(`assembly-board:v2:product-a-chronology`), so renaming from a bare id to a
prefixed id does not migrate old saved layout.

Refreshing a board id that already belongs to a different `repo` value fails
closed instead of overwriting. Existing local-only `public/boards.json` entries
are not auto-imported; re-register with `--prefix` into the YAML registry.

## Slice boards

Prefer catalog slice projections when a full-module board is too dense. Each
slice keeps owned packages as full nodes and turns external neighbors into
boundary stubs (dashed cards with `[Slice:…]` / `[Library:…]` / `[Unowned]`).
Cables without a matching catalog binding render amber as missing-binding debt.

```bash
# One slice board (board id usually matches the slice id):
typology boards register /path/to/consumer-repo chronology \
  --module engine --slice chronology \
  --catalog /path/to/consumer-repo/.typology/typology.yaml \
  --label "Chronology" \
  --viewer /path/to/typology/viewer/cable-board/public

# Every confirmed catalog slice:
typology boards register /path/to/consumer-repo --all-slices \
  --module engine --catalog /path/to/consumer-repo/.typology/typology.yaml \
  --viewer /path/to/typology/viewer/cable-board/public

# Or from the Typology module against the tiny fixture:
make cable-board-slices
```

Machine-only harvest (no XDG registry) still uses:

```bash
typology assembly-graph REPO --catalog PATH --slice SLICE --out PATH
typology assembly-graph REPO --catalog PATH --all-slices --out-dir DIR
```

## Behavior

- Arrow direction: **importer → imported**
- Click a package to detangle its 1-hop neighborhood; click canvas to restore
- Layers filter role-kind cables (`imports`, `fills_dto`, …); default layer is Imports
- Risk marks: cycles, mesh density, wrong-way role-layer cables
- Boundary stubs and missing-binding cables appear on slice-projected boards
- Layout, selection, and cable nudges are saved per board id in localStorage
- Repo filter groups the board switcher when `boards[].repo` is set
