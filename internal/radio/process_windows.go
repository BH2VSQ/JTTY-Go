//go:build windows

package radio

import (
	"context"
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW. Together with HideWindow this prevents
// console-subsystem Hamlib utilities from creating visible Command Prompt
// windows when launched by the GUI application.
const createNoWindow = 0x08000000

func newHamlibCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	configureHiddenHamlibProcess(cmd)
	return cmd
}

func newHamlibCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureHiddenHamlibProcess(cmd)
	return cmd
}

func configureHiddenHamlibProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
