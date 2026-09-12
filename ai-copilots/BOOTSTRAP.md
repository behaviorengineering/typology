# BOOTSTRAP — Typology ai-copilots

**Audience:** Any AI agent (Cursor, GitHub Copilot, Claude Code, Codex) in a workspace that depends on or checks out this module.

**Goal:** Wire host IDE discovery to canonical content under `ai-copilots/`. Optionally refresh content. **MUST NOT** copy skill bodies unless symlinks or junctions fail and the user approves copy fallback.

**Module path:** `github.com/behaviorengineering/typology`

---

## When to run

| Mode | Phases |
|------|--------|
| **Wire only** | 0 → 2 → 3 → 4 |
| **Refresh content + wire** | 0 → 1 → 2 → 3 → 4 |

---

## Phase 0 — Resolve module root

From a Go module that requires `github.com/behaviorengineering/typology` (or this checkout):

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology)"
test -d "$MOD/ai-copilots/skills" || { echo "missing ai-copilots under $MOD"; exit 1; }
echo "Module Dir: $MOD"
```

If `go list` is unavailable, use a known nested checkout path only when it clearly contains `ai-copilots/`. MUST NOT invent a path.

Re-run wire after module version bumps (cache Dir can change).

---

## Phase 1 — Refresh content (optional)

Edit only files under `$MOD/ai-copilots/`. For authoring standards, load the host pack skill `author-ai-copilots` and `agent-smith` when available.

Target tree:

```text
ai-copilots/
  README.md
  BOOTSTRAP.md
  skills/
    README.md
    journey/SKILL.md
    docs/SKILL.md
    catalog/SKILL.md
    cli/SKILL.md
```

---

## Phase 2 — Ask IDE and OS if unknown

1. IDE: Cursor, GitHub Copilot, Claude Code, Codex
2. OS: macOS/Linux symlink vs Windows junction/copy
3. Workspace: Typology alone vs nested under a parent monorepo vs dependency-only

---

## Phase 3 — Wire discovery

Canonical skill trees under `$MOD`:

| Host skill name | Path under `$MOD` |
|-----------------|-------------------|
| `typology-journey` | `ai-copilots/skills/journey/` |
| `typology-docs` | `ai-copilots/skills/docs/` |
| `typology-catalog` | `ai-copilots/skills/catalog/` |
| `typology-cli` | `ai-copilots/skills/cli/` |

Discovery paths:

| IDE | Skills |
|-----|--------|
| Cursor | `.cursor/skills/**/SKILL.md` |
| GitHub Copilot | `.github/skills/**/SKILL.md` |
| Claude Code | `.claude/skills/**/SKILL.md` |
| Codex | `.codex/skills/**/SKILL.md` |

**Cursor example (macOS/Linux)** from the **host workspace root**:

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology)"
mkdir -p .cursor/skills
ln -snf "$MOD/ai-copilots/skills/journey" .cursor/skills/typology-journey
ln -snf "$MOD/ai-copilots/skills/docs" .cursor/skills/typology-docs
ln -snf "$MOD/ai-copilots/skills/catalog" .cursor/skills/typology-catalog
ln -snf "$MOD/ai-copilots/skills/cli" .cursor/skills/typology-cli
```

When the workspace root is the Typology module itself, relative links are fine:

```bash
mkdir -p .cursor/skills
ln -snf ../ai-copilots/skills/journey .cursor/skills/typology-journey
ln -snf ../ai-copilots/skills/docs .cursor/skills/typology-docs
ln -snf ../ai-copilots/skills/catalog .cursor/skills/typology-catalog
ln -snf ../ai-copilots/skills/cli .cursor/skills/typology-cli
```

**Windows:** prefer junction or developer-mode symlink; copy fallback only with user approval.

**Idempotency:** skip if the link already resolves to the canonical path; ask before overwriting stale copies.

**Parent monorepo:** MUST NOT edit parent skill indexes unless the user asks.

---

## Phase 4 — Verify

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology)"
ls -la .cursor/skills/typology-journey .cursor/skills/typology-cli
test -f .cursor/skills/typology-journey/SKILL.md
test -f "$MOD/ai-copilots/BOOTSTRAP.md"
```

Ask the user before committing host wiring (`.cursor/`, `.github/`, etc.).
