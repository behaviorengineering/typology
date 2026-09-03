# Typology skills

Portable instructions for any coding agent that builds with this library. Canonical path is this directory. Hosts MAY symlink `journey/`, `docs/`, `catalog/`, and `cli/` into their own skills folder; they MUST NOT treat a copy as source of truth.

| Skill | Load when |
|-------|-----------|
| [journey/SKILL.md](journey/SKILL.md) | First catalog in a repo, onboard, walk slices, refine desired architecture, or resume `.typology/typology-journey.md` |
| [docs/SKILL.md](docs/SKILL.md) | Fill or evaluate develop DocPages and program leaves; journey phase `docs`; write slice docs |
| [catalog/SKILL.md](catalog/SKILL.md) | Authoring or changing `.typology/typology.yaml`, `catalog.Typology` in Go, subprograms, actuators, opRuns, or bindings |
| [cli/SKILL.md](cli/SKILL.md) | Running `typology discover`, `emit`, `validate`, `show`, or `remediate` |

Symlink recipe and consumer layout: [../AGENTS.md](../AGENTS.md). Human pitch: [../README.md](../README.md) (§ Consumer setup). Types: [../catalog/types.go](../catalog/types.go).
