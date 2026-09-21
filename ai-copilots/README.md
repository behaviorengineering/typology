# Typology ai-copilots

Portable agent pack for Typology: journey, catalog, CLI, docs, and cable-board skills (`assembly-graph` / `boards register`), plus Cursor rules under `rules/` (slice domain language).

Canonical source lives here. Wiring is done by the AI copilot when you ask it to execute [BOOTSTRAP.md](BOOTSTRAP.md). MUST NOT move these bodies into cursor-packs; hosts link here.

**Minimal prompt:**

> Wire Typology ai-copilots using BOOTSTRAP.md

**Example (Cursor, dependency or nested checkout):**

> Execute `ai-copilots/BOOTSTRAP.md` in wire mode. Cursor on macOS. Resolve module with `go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology`.

Skills index: [skills/README.md](skills/README.md). Entry for agents: [../AGENTS.md](../AGENTS.md).
