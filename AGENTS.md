# Agents

This module is a Go library and CLI. Humans read [README.md](README.md).

**Load these skills before you write a catalog, call the library, or run the CLI:**

1. [skills/README.md](skills/README.md) (index)
2. [skills/catalog/SKILL.md](skills/catalog/SKILL.md) (model and YAML/Go catalog)
3. [skills/cli/SKILL.md](skills/cli/SKILL.md) (discover, emit, validate, remediate)

Worked catalog: [testdata/tiny-module/architecture/typology.yaml](testdata/tiny-module/architecture/typology.yaml). Types: [catalog/types.go](catalog/types.go).

## Optional: symlink into your host

Skills ship in this repo under `skills/`. They are not bound to any one agent product. If your host only auto-loads a local skills directory, you MAY symlink the folders you need:

```bash
# from the consumer repo, after this module is on disk (module cache, vendor, or submodule)
ln -s "$TYPOLOGY_ROOT/skills/catalog" "$HOST_SKILLS/typology-catalog"
ln -s "$TYPOLOGY_ROOT/skills/cli" "$HOST_SKILLS/typology-cli"
```

`$HOST_SKILLS` is whatever that host already uses (for example `.cursor/skills`, `.claude/skills`, `.codex/skills`). MUST keep the link pointing at this module's `skills/` tree so updates follow the pin. MUST NOT copy the files into the host tree unless the host cannot follow symlinks.

You MAY skip linking and Read the `SKILL.md` files in place from `AGENTS.md`.
