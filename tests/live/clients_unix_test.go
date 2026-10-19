//go:build live && clients && !windows

package live

import (
	"os/exec"
	"syscall"
	"time"
)

// inGroup starts a client in a process group of its own, so ending it ends
// what it started too: Goose's backend and every mm-mcp a client ran.
func inGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func endGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = command.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		<-done
	}
}
