//go:build !windows

package radio

import (
	"context"
	"os/exec"
)

func newHamlibCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func newHamlibCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
