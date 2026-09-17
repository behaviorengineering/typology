#!/usr/bin/env bash
# Load a cable board JSON into public/assembly-graph.json for the Vite viewer.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MOD_ROOT="$(cd "$ROOT/../.." && pwd)"
REPO="${1:-$MOD_ROOT/testdata/tiny-module}"
OUT="$ROOT/public/assembly-graph.json"
BIN="${TYPOLOGY_BIN:-}"

if [[ -z "$BIN" ]]; then
  if [[ -x "$MOD_ROOT/bin/typology" ]]; then
    BIN="$MOD_ROOT/bin/typology"
  else
    (cd "$MOD_ROOT" && make build)
    BIN="$MOD_ROOT/bin/typology"
  fi
fi

mkdir -p "$(dirname "$OUT")"
"$BIN" assembly-graph "$REPO" --out "$OUT"
echo "viewer ready: $OUT"
echo "run: cd $ROOT && npm install && npm run dev"
