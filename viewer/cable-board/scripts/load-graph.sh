#!/usr/bin/env bash
# Register a cable board JSON for the Vite viewer.
#
# Legacy single board (no registry touched):
#   load-graph.sh REPO
#
# Named board (registry entry under public/boards/<id>/):
#   load-graph.sh REPO BOARD_ID [--module PATH] [--label TEXT] [--make-default]
#
# Slice-projected board (catalog owns[] + boundary stubs):
#   load-graph.sh REPO BOARD_ID --slice SLICE [--catalog PATH] [--module PATH] [--label TEXT]
#
# Register every confirmed catalog slice (BOARD_ID ignored; slice ids used):
#   load-graph.sh REPO --all-slices [--catalog PATH] [--module PATH]
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
SLICE=""
CATALOG=""
ALL_SLICES=0

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
    --slice)
      SLICE="${2:?load-graph.sh: --slice requires an id}"
      shift 2
      ;;
    --catalog)
      CATALOG="${2:?load-graph.sh: --catalog requires a path}"
      shift 2
      ;;
    --all-slices)
      ALL_SLICES=1
      shift
      ;;
    *)
      echo "load-graph.sh: unknown flag $1" >&2
      echo "usage: load-graph.sh REPO [BOARD_ID] [--module PATH] [--label TEXT] [--make-default] [--slice SLICE] [--catalog PATH] [--all-slices]" >&2
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

register_board() {
  local board_id="$1"
  local board_label="$2"
  local out="$3"
  local make_default="$4"
  local manifest="$ROOT/public/boards.json"
  BOARDS_JSON="$manifest" BOARD_ID="$board_id" BOARD_LABEL="$board_label" \
    BOARD_GRAPH="/boards/$board_id/assembly-graph.json" MAKE_DEFAULT="$make_default" \
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
  if [[ "$make_default" == "1" ]]; then
    cp "$out" "$ROOT/public/assembly-graph.json"
    echo "default board copy refreshed: $ROOT/public/assembly-graph.json"
  fi
  echo "viewer ready: $out"
  echo "open: http://localhost:5173/?board=$board_id"
}

run_assembly() {
  local out="$1"
  shift
  mkdir -p "$(dirname "$out")"
  "$BIN" assembly-graph "$REPO" "$@" --out "$out"
}

if [[ "$ALL_SLICES" == "1" ]]; then
  if [[ -n "$SLICE" ]]; then
    echo "load-graph.sh: use --slice or --all-slices, not both" >&2
    exit 2
  fi
  CATALOG="${CATALOG:-$REPO/.typology/typology.yaml}"
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  ARGS=(--all-slices --catalog "$CATALOG" --out-dir "$TMP_DIR")
  if [[ -n "$MODULE" ]]; then
    ARGS+=(--module "$MODULE")
  fi
  "$BIN" assembly-graph "$REPO" "${ARGS[@]}"
  # Register each slice board written under TMP_DIR/<id>/assembly-graph.json
  for slice_dir in "$TMP_DIR"/*; do
    [[ -d "$slice_dir" ]] || continue
    slice_id="$(basename "$slice_dir")"
    if ! [[ "$slice_id" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
      echo "load-graph.sh: skipping unsafe slice id $slice_id" >&2
      continue
    fi
    src="$slice_dir/assembly-graph.json"
    [[ -f "$src" ]] || continue
    out="$ROOT/public/boards/$slice_id/assembly-graph.json"
    mkdir -p "$(dirname "$out")"
    cp "$src" "$out"
    register_board "$slice_id" "$slice_id" "$out" "0"
  done
  echo "run: cd $ROOT && npm install && npm run dev"
  exit 0
fi

if [[ -z "$BOARD_ID" ]]; then
  OUT="$ROOT/public/assembly-graph.json"
  mkdir -p "$(dirname "$OUT")"
  ARGS=()
  if [[ -n "$MODULE" ]]; then
    ARGS+=(--module "$MODULE")
  fi
  if [[ -n "$SLICE" ]]; then
    CATALOG="${CATALOG:-$REPO/.typology/typology.yaml}"
    ARGS+=(--catalog "$CATALOG" --slice "$SLICE")
  fi
  run_assembly "$OUT" "${ARGS[@]}"
  echo "viewer ready: $OUT"
  echo "run: cd $ROOT && npm install && npm run dev"
  exit 0
fi

if ! [[ "$BOARD_ID" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  echo "load-graph.sh: BOARD_ID must match ^[a-z0-9][a-z0-9-]*\$ (got $BOARD_ID)" >&2
  exit 2
fi
if [[ -z "$LABEL" ]]; then
  if [[ -n "$SLICE" ]]; then
    LABEL="$SLICE"
  else
    LABEL="$BOARD_ID"
  fi
fi

OUT="$ROOT/public/boards/$BOARD_ID/assembly-graph.json"
ARGS=()
if [[ -n "$MODULE" ]]; then
  ARGS+=(--module "$MODULE")
fi
if [[ -n "$SLICE" ]]; then
  CATALOG="${CATALOG:-$REPO/.typology/typology.yaml}"
  ARGS+=(--catalog "$CATALOG" --slice "$SLICE")
fi
run_assembly "$OUT" "${ARGS[@]}"
register_board "$BOARD_ID" "$LABEL" "$OUT" "$MAKE_DEFAULT"
echo "run: cd $ROOT && npm install && npm run dev"
