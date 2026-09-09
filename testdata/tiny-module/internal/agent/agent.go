package agent

import (
	"example.com/tiny/internal/board"
	"example.com/tiny/internal/ledger"
	"example.com/tiny/internal/runner"
)

// Investigator coordinates a runner and ledger without importing os/exec itself.
type Investigator struct {
	Run *runner.Runner
}

// New builds an investigation orchestrator.
func New(r *runner.Runner) *Investigator {
	return &Investigator{Run: r}
}

// Investigate shells out through the runner and fills board rows.
func (i *Investigator) Investigate() []board.Item {
	if i.Run != nil {
		_ = i.Run.Run("true")
	}
	_ = ledger.Balance()
	return []board.Item{{ID: "branch", Title: "candidate"}}
}
