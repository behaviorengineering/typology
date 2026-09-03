# Agents

This module is a Go library and CLI. Humans read [README.md](README.md).

Typology guides agents on how to put code together: slices are bounded contexts, components are packages, bindings are allowed couplings. It is not a police tool for every edit. Day-to-day: update the catalog first, implement to match it, then validate.

**Load these skills before you write a catalog, call the library, or run the CLI:**

1. [skills/README.md](skills/README.md) (index)
2. [skills/journey/SKILL.md](skills/journey/SKILL.md) (first map, onboard, resume `.typology/typology-journey.md`)
3. [skills/docs/SKILL.md](skills/docs/SKILL.md) (fill and evaluate develop DocPages and program leaves)
4. [skills/catalog/SKILL.md](skills/catalog/SKILL.md) (model and YAML/Go catalog)
5. [skills/cli/SKILL.md](skills/cli/SKILL.md) (discover, emit, validate, remediate)

Worked catalog: [testdata/tiny-module/.typology/typology.yaml](testdata/tiny-module/.typology/typology.yaml). Types: [catalog/types.go](catalog/types.go).

## Consumer repos (other libraries)

When another Go library adopts Typology, `typology emit REPO` installs the same agent setup in that repo:

- `.typology/typology.yaml` — confirmed catalog
- `.typology/README.md` — skills, commands, catalog-first workflow
- `AGENTS.md` — pointer to `.typology/README.md` (created or appended; keeps existing content)
- `tmp/typology/` — discover drafts only

After emit, agents in that repo start at `AGENTS.md` → `.typology/README.md` → skills. Full layout: [README.md](README.md) § Consumer setup.

## Optional: symlink into your host

Skills ship in this repo under `skills/`. They are not bound to any one agent product. If your host only auto-loads a local skills directory, you MAY symlink the folders you need:

```bash
# from the consumer repo, after this module is on disk (module cache, vendor, or submodule)
ln -s "$TYPOLOGY_ROOT/skills/catalog" "$HOST_SKILLS/typology-catalog"
ln -s "$TYPOLOGY_ROOT/skills/cli" "$HOST_SKILLS/typology-cli"
ln -s "$TYPOLOGY_ROOT/skills/journey" "$HOST_SKILLS/typology-journey"
ln -s "$TYPOLOGY_ROOT/skills/docs" "$HOST_SKILLS/typology-docs"
```

`$HOST_SKILLS` is whatever that host already uses (for example `.cursor/skills`, `.claude/skills`, `.codex/skills`). MUST keep the link pointing at this module's `skills/` tree so updates follow the pin. MUST NOT copy the files into the host tree unless the host cannot follow symlinks.

You MAY skip linking and Read the `SKILL.md` files in place from `AGENTS.md`.
