# Agents

This module is a Go library and CLI. Humans read [README.md](README.md).

Typology guides agents on how to put code together: slices are bounded contexts, components are packages, bindings are allowed couplings. It is not a police tool for every edit. Day-to-day: update the catalog first, implement to match it, then validate.

One repository owns one Typology catalog and its architecture documentation. In a multi-module repository, `go.work` is dependency context, not Typology scope. The catalog's `scope.modules` list is authoritative; do not run a workspace-wide scan without an explicit scope.

**Load these skills before you write a catalog, call the library, or run the CLI:**

1. [ai-copilots/skills/README.md](ai-copilots/skills/README.md) (index)
2. [ai-copilots/skills/journey/SKILL.md](ai-copilots/skills/journey/SKILL.md) (first map, onboard, resume `.typology/typology-journey.md`)
3. [ai-copilots/skills/docs/SKILL.md](ai-copilots/skills/docs/SKILL.md) (fill and evaluate develop DocPages and program leaves)
4. [ai-copilots/skills/catalog/SKILL.md](ai-copilots/skills/catalog/SKILL.md) (model and YAML/Go catalog)
5. [ai-copilots/skills/cli/SKILL.md](ai-copilots/skills/cli/SKILL.md) (discover, emit, validate, remediate)

Worked catalog: [testdata/tiny-module/.typology/typology.yaml](testdata/tiny-module/.typology/typology.yaml). Types: [catalog/types.go](catalog/types.go).

## Consumer repos (other libraries)

When another Go library adopts Typology, `typology emit REPO` installs the same agent setup in that repo:

- `.typology/typology.yaml` — confirmed catalog
- `.typology/README.md` — skills, commands, catalog-first workflow
- `AGENTS.md` — pointer to `.typology/README.md` (created or appended; keeps existing content)
- `tmp/typology/` — discover drafts only

Before running Typology commands in a consumer, register the CLI as a Go tool:

```bash
go run github.com/behaviorengineering/typology/cmd/typology@v0.0.5 init .
go tool typology version
```

`init` updates the selected consumer module's `go.mod` and `go.sum`. It does not add the CLI to application imports or binaries. If `go.work` covers more than one module, pass the module path with `--module`; the command fails rather than choosing one.

After emit, agents in that repo start at `AGENTS.md` → `.typology/README.md` → skills. Full layout: [README.md](README.md) § Consumer setup. Use `--module PATH` only for first-map or focused runs; it does not transfer catalog ownership between repositories.

## Wire host discovery

Skills ship under [ai-copilots/](ai-copilots/). They are not bound to any one agent product. Execute [ai-copilots/BOOTSTRAP.md](ai-copilots/BOOTSTRAP.md) in **wire mode** to symlink into `.cursor/skills`, `.github/skills`, `.claude/skills`, or `.codex/skills`.

Resolve the module root when Typology is only a Go dependency:

```bash
go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology
```

Manual symlink recipe (same paths BOOTSTRAP uses):

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/typology)"
ln -snf "$MOD/ai-copilots/skills/catalog" "$HOST_SKILLS/typology-catalog"
ln -snf "$MOD/ai-copilots/skills/cli" "$HOST_SKILLS/typology-cli"
ln -snf "$MOD/ai-copilots/skills/journey" "$HOST_SKILLS/typology-journey"
ln -snf "$MOD/ai-copilots/skills/docs" "$HOST_SKILLS/typology-docs"
```

`$HOST_SKILLS` is whatever that host already uses (for example `.cursor/skills`, `.claude/skills`, `.codex/skills`). MUST keep the link pointing at this module's `ai-copilots/skills/` tree so updates follow the pin. MUST NOT copy the files into the host tree unless the host cannot follow symlinks.

You MAY skip linking and Read the `SKILL.md` files in place from this file.
