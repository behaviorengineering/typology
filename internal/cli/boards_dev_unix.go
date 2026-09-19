//go:build unix

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func setServeForegroundProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interruptServeChild(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process == nil {
		return
	}
	// Signal the whole process group (npm + vite), not only the npm wrapper.
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, sig.(syscall.Signal))
}

func isServeInterrupt(err error) bool {
	if err == nil {
		return false
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok {
			return status.Signaled()
		}
	}
	return false
}
