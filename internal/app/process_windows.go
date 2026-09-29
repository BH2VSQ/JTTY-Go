//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// newHiddenCommand starts a console-subsystem helper without creating a
// visible console window. JTTY-Go is a GUI application, so helper processes
// such as rigctld must never steal focus or flash a Command Prompt window.
func newHiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd
}
