#!/usr/bin/env bash
# Smoke: XDG YAML registry + prefixed boards + materialize + collision guard.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VIEWER="$ROOT/viewer/cable-board"
SMOKE_REPO="$ROOT/testdata/tiny-module"
SMOKE_CATALOG="$SMOKE_REPO/.typology/typology.yaml"
PUBLIC="$VIEWER/public"
BIN="$ROOT/bin/typology"

CFG="$(mktemp -d)"
DATA="$(mktemp -d)"
BACKUP_MANIFEST=""
cleanup() {
  if [[ -n "$BACKUP_MANIFEST" && -f "$BACKUP_MANIFEST" ]]; then
    cp "$BACKUP_MANIFEST" "$PUBLIC/boards.json"
  fi
  rm -rf "$CFG" "$DATA" \
    "$PUBLIC/boards/alpha-billing" "$PUBLIC/boards/beta-billing" \
    "$PUBLIC/boards/alpha-ledger" "$PUBLIC/boards/beta-ledger" \
    "$PUBLIC/boards/alpha-delivery" "$PUBLIC/boards/beta-delivery"
  rm -f "${BACKUP_MANIFEST:-}"
}
trap cleanup EXIT

export TYPOLOGY_CONFIG_DIR="$CFG"
export TYPOLOGY_DATA_DIR="$DATA"
export PUBLIC

if [[ ! -x "$BIN" ]]; then
  (cd "$ROOT" && make build)
fi

if [[ -f "$PUBLIC/boards.json" ]]; then
  BACKUP_MANIFEST="$(mktemp)"
  cp "$PUBLIC/boards.json" "$BACKUP_MANIFEST"
fi

"$BIN" boards register "$SMOKE_REPO" --all-slices --prefix alpha --catalog "$SMOKE_CATALOG" --viewer "$PUBLIC"
"$BIN" boards register "$SMOKE_REPO" --all-slices --prefix beta --catalog "$SMOKE_CATALOG" --viewer "$PUBLIC"

python3 - <<'PY'
import json
import os
from pathlib import Path

cfg = Path(os.environ["TYPOLOGY_CONFIG_DIR"])
data = Path(os.environ["TYPOLOGY_DATA_DIR"])
public = Path(os.environ["PUBLIC"])
yaml_path = cfg / "boards.yaml"
text = yaml_path.read_text(encoding="utf-8")
for prefix in ("alpha", "beta"):
    for slice_id in ("billing", "ledger", "delivery"):
        board_id = f"{prefix}-{slice_id}"
        if f"id: {board_id}" not in text:
            raise SystemExit(f"yaml missing {board_id}:\n{text}")
        share = data / "boards" / board_id / "assembly-graph.json"
        if not share.is_file():
            raise SystemExit(f"missing share graph {share}")
        pub = public / "boards" / board_id / "assembly-graph.json"
        if not pub.is_file():
            raise SystemExit(f"missing materialized graph {pub}")
manifest = json.loads((public / "boards.json").read_text(encoding="utf-8"))
ids = {b["id"] for b in manifest["boards"]}
for prefix in ("alpha", "beta"):
    for slice_id in ("billing", "ledger", "delivery"):
        board_id = f"{prefix}-{slice_id}"
        if board_id not in ids:
            raise SystemExit(f"manifest missing {board_id}")
        entry = next(b for b in manifest["boards"] if b["id"] == board_id)
        if entry.get("repo") != prefix:
            raise SystemExit(f"{board_id} repo={entry.get('repo')!r}")
keys = {f"assembly-board:v2:{i}" for i in ids}
if len(keys) != len(ids):
    raise SystemExit("storage key collision")
print("xdg-prefix-boards smoke: ok")
PY

# Collision: claim alpha-billing under a different repo in YAML, then refuse refresh.
python3 - <<'PY'
from pathlib import Path
import os
path = Path(os.environ["TYPOLOGY_CONFIG_DIR"]) / "boards.yaml"
lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
out = []
in_alpha_billing = False
patched = False
for line in lines:
    if line.startswith("    - id: alpha-billing"):
        in_alpha_billing = True
    elif line.startswith("    - id:"):
        in_alpha_billing = False
    if in_alpha_billing and line.strip() == "repo: alpha":
        out.append(line.replace("repo: alpha", "repo: intruder", 1))
        patched = True
        in_alpha_billing = False
        continue
    out.append(line)
if not patched:
    raise SystemExit("could not patch repo in yaml:\n" + "".join(lines))
path.write_text("".join(out), encoding="utf-8")
PY

if "$BIN" boards register "$SMOKE_REPO" billing \
  --slice billing --prefix alpha --catalog "$SMOKE_CATALOG" --viewer "$PUBLIC"; then
  echo "expected collision refusal for alpha-billing claimed by repo intruder" >&2
  exit 1
fi

echo "xdg-prefix-boards collision guard: ok"
