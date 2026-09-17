# Typology cable board viewer

Interactive ComfyUI-style board for the portable cable board JSON produced by
`typology assembly-graph` (`tmp/typology/assembly-graph.json` by default).

This viewer is the human surface for the [cable-board skill](../../ai-copilots/skills/cable-board/SKILL.md).
It is product-neutral: any consumer repo can harvest a graph and open this UI.

## Run

From the Typology module root:

```bash
make build
make cable-board-sample
cd viewer/cable-board
npm install
npm run dev
```

Open the printed URL (default http://localhost:5173). The page loads the
default board from `public/boards.json`.

Default `load-graph.sh` without a board id harvests `testdata/tiny-module`
into the legacy `public/assembly-graph.json` only.

## Boards

One server can serve many named boards. Each board keeps a stable id
(lowercase, digits, hyphens) that appears in `?board=` URLs, so different
windows or tabs can open different boards side by side.

```bash
# Register (or refresh) a named board without touching the others:
./viewer/cable-board/scripts/load-graph.sh /path/to/consumer-repo engine \
  --module engine --label "Consilium engine"

# Make it the default board (also refreshes legacy public/assembly-graph.json):
./viewer/cable-board/scripts/load-graph.sh /path/to/consumer-repo engine \
  --module engine --label "Consilium engine" --make-default
```

Then open `http://localhost:5173/?board=engine` in one window and
`http://localhost:5173/?board=tiny-module` in another. The header switcher
moves between boards and keeps the board in the URL.

Registry file: `public/boards.json` (`defaultBoard` plus `boards[]` with
`id`, `label`, and `graph` path). Only the tiny-module sample is committed;
consumer boards are local. The page loads `/assembly-graph.json` from
`public/` when no registry exists. Override any board with
`?src=/other.json`.

## Behavior

- Arrow direction: **importer → imported**
- Click a package to detangle its 1-hop neighborhood; click canvas to restore
- Layers filter role-kind cables (`imports`, `fills_dto`, …)
- Risk marks: cycles, mesh density, wrong-way role-layer cables

## Generate without the helper

```bash
typology assembly-graph REPO --out viewer/cable-board/public/assembly-graph.json
```
