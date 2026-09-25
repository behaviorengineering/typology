package cli

import "fmt"

func agentOperatingGuide() string {
	return fmt.Sprintf(`typology %s — Go repo architecture catalogs, boards, and validation

ROLE & BOUNDARIES
  Discovers packages, emits catalogs, validates assembly graphs, and runs boards.
  Host Consilium uses typology export/validate via consilium-pii typology commands.

AGENT OPERATING GUIDE
  Read AGENTS.md and ai-copilots/ in this module before rewriting catalogs.
  Operator skills: ai-copilots/skills/typology-cli/SKILL.md (host mirror in .cursor/skills)
  Flow: discover draft → validate → emit → boards refresh.

COMMANDS BY RISK & LIFECYCLE
  Inspect & Validate
    validate    Catalog and graph validators
    show        Slice or component detail
    version     Build identity

  Plan & Preview
    discover    Draft catalog without committing host pins

  Execute & Mutate
    emit        Write generated artefacts
    remediate   Apply guided catalog fixes

AUTOMATION RULES FOR AGENTS
  - Validate before emit on shared catalogs.
  - Unknown commands exit non-zero; bare invoke exits 0 with this guide.
  - Full flag reference: typology help
`, reportVersion())
}
