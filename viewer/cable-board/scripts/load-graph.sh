#!/usr/bin/env bash
# Register a cable board JSON for the Vite viewer.
#
# Legacy single board (no registry touched):
#   load-graph.sh REPO
#
# Named board (registry entry under public/boards/<id>/):
#   load-graph.sh REPO BOARD_ID [--module PATH] [--label TEXT] [--make-default]
#
# BOARD_ID is stable: lowercase, digits, hyphens. It appears in `?board=`
# URLs, storage keys, and directory names. One server can serve many boards;
# registering a board never deletes the others.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MOD_ROOT="$(cd "$ROOT/../.." && pwd)"
REPO="${1:-$MOD_ROOT/testdata/tiny-module}"
shift || true

BOARD_ID=""
MODULE=""
LABEL=""
MAKE_DEFAULT=0
if [[ $# -gt 0 && "$1" != --* ]]; then
  BOARD_ID="$1"
  shift
fi
while [[ $# -gt 0 ]]; do
  case "$1" in
    --module)
      MODULE="${2:?load-graph.sh: --module requires a path}"
      shift 2
      ;;
    --label)
      LABEL="${2:?load-graph.sh: --label requires text}"
      shift 2
      ;;
    --make-default)
      MAKE_DEFAULT=1
      shift
      ;;
    *)
      echo "load-graph.sh: unknown flag $1" >&2
      echo "usage: load-graph.sh REPO [BOARD_ID] [--module PATH] [--label TEXT] [--make-default]" >&2
      exit 2
      ;;
  esac
done

BIN="${TYPOLOGY_BIN:-}"
if [[ -z "$BIN" ]]; then
  if [[ -x "$MOD_ROOT/bin/typology" ]]; then
    BIN="$MOD_ROOT/bin/typology"
  else
    (cd "$MOD_ROOT" && make build)
    BIN="$MOD_ROOT/bin/typology"
  fi
fi

if [[ -z "$BOARD_ID" ]]; then
  OUT="$ROOT/public/assembly-graph.json"
  mkdir -p "$(dirname "$OUT")"
  if [[ -n "$MODULE" ]]; then
    "$BIN" assembly-graph "$REPO" --module "$MODULE" --out "$OUT"
  else
    "$BIN" assembly-graph "$REPO" --out "$OUT"
  fi
  echo "viewer ready: $OUT"
  echo "run: cd $ROOT && npm install && npm run dev"
  exit 0
fi

if ! [[ "$BOARD_ID" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  echo "load-graph.sh: BOARD_ID must match ^[a-z0-9][a-z0-9-]*\$ (got $BOARD_ID)" >&2
  exit 2
fi
if [[ -z "$LABEL" ]]; then
  LABEL="$BOARD_ID"
fi

OUT="$ROOT/public/boards/$BOARD_ID/assembly-graph.json"
MANIFEST="$ROOT/public/boards.json"
mkdir -p "$(dirname "$OUT")"
if [[ -n "$MODULE" ]]; then
  "$BIN" assembly-graph "$REPO" --module "$MODULE" --out "$OUT"
else
  "$BIN" assembly-graph "$REPO" --out "$OUT"
fi

BOARDS_JSON="$MANIFEST" BOARD_ID="$BOARD_ID" BOARD_LABEL="$LABEL" \
  BOARD_GRAPH="/boards/$BOARD_ID/assembly-graph.json" MAKE_DEFAULT="$MAKE_DEFAULT" \
  python3 - <<'PY'
import json
import os

path = os.environ["BOARDS_JSON"]
board_id = os.environ["BOARD_ID"]
manifest = {"boards": []}
if os.path.exists(path):
    with open(path, encoding="utf-8") as f:
        manifest = json.load(f)
if not isinstance(manifest.get("boards"), list):
    raise SystemExit(f"boards manifest has non-list boards: {path}")
entry = {
    "id": board_id,
    "label": os.environ["BOARD_LABEL"],
    "graph": os.environ["BOARD_GRAPH"],
}
boards = manifest["boards"]
for i, existing in enumerate(boards):
    if isinstance(existing, dict) and existing.get("id") == board_id:
        boards[i] = entry
        break
else:
    boards.append(entry)
ids = [b.get("id") for b in boards if isinstance(b, dict)]
if os.environ.get("MAKE_DEFAULT") == "1" or manifest.get("defaultBoard") not in ids:
    manifest["defaultBoard"] = board_id
manifest["boards"] = boards
with open(path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
print(f"registry updated: {path} (board {board_id})")
PY

if [[ "$MAKE_DEFAULT" == "1" ]]; then
  cp "$OUT" "$ROOT/public/assembly-graph.json"
  echo "default board copy refreshed: $ROOT/public/assembly-graph.json"
fi

echo "viewer ready: $OUT"
echo "open: http://localhost:5173/?board=$BOARD_ID"
echo "run: cd $ROOT && npm install && npm run dev"
