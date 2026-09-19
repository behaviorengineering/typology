//go:build !unix

package cli

import (
	"os"
	"os/exec"
)

func setServeForegroundProcGroup(cmd *exec.Cmd) {}

func interruptServeChild(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(sig)
}

func isServeInterrupt(err error) bool {
	return false
}
