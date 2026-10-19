//go:build live && clients && windows

package live

import (
	"os/exec"
	"strconv"
)

// inGroup leaves the client as it starts: Windows ends what it started with it
// through taskkill's tree.
func inGroup(*exec.Cmd) {}

func endGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid)).Run()
	_ = command.Wait()
}
