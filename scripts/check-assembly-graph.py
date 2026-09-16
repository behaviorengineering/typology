#!/usr/bin/env python3
"""Fail closed if assembly-graph JSON drifts from the portable contract."""
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    doc = json.load(f)

nodes = doc.get("nodes") or []
edges = doc.get("edges") or []
if not nodes or not edges:
    raise SystemExit("assembly graph empty")

node0 = nodes[0]
for key in ("id", "path", "inDegree", "outDegree", "imports", "importedBy", "isHub", "isLeaf", "layer"):
    if key not in node0:
        raise SystemExit(f"node missing {key}")

edge0 = edges[0]
for key in ("id", "source", "target"):
    if key not in edge0:
        raise SystemExit(f"edge missing {key}")

if not any(e.get("roleKind") for e in edges):
    raise SystemExit("expected a roleKind")
