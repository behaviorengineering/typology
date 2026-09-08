package runner

import "os/exec"

// Runner shells out to external processes without product knowledge.
type Runner struct{}

// New builds a process runner.
func New() *Runner {
	return &Runner{}
}

// Run executes a command.
func (r *Runner) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
