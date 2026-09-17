# Typology cable board viewer

Interactive ComfyUI-style board for the portable cable board JSON produced by
`typology assembly-graph` (`tmp/typology/assembly-graph.json` by default).

This viewer is the human surface for the [cable-board skill](../../ai-copilots/skills/cable-board/SKILL.md).
It is product-neutral: any consumer repo can harvest a graph and open this UI.

## Run

From the Typology module root:

```bash
make build
./viewer/cable-board/scripts/load-graph.sh /path/to/consumer-repo
cd viewer/cable-board
npm install
npm run dev
```

Open the printed URL (default http://localhost:5173). The page loads
`/assembly-graph.json` from `public/`. Override with `?src=/other.json`.

Default `load-graph.sh` without args harvests `testdata/tiny-module`.

## Behavior

- Arrow direction: **importer → imported**
- Click a package to detangle its 1-hop neighborhood; click canvas to restore
- Layers filter role-kind cables (`imports`, `fills_dto`, …)
- Risk marks: cycles, mesh density, wrong-way role-layer cables

## Generate without the helper

```bash
typology assembly-graph REPO --out viewer/cable-board/public/assembly-graph.json
```
