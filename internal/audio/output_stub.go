//go:build !windows

package audio

import (
	"context"
	"fmt"
)

type stubOutput struct{}

func NewOutput() Output { return stubOutput{} }
func (stubOutput) Open(context.Context, OutputConfig) error {
	return fmt.Errorf("Windows audio output is only available on Windows")
}
func (stubOutput) Play(context.Context, []float32, int) error {
	return fmt.Errorf("Windows audio output is only available on Windows")
}
func (stubOutput) Stop() error  { return nil }
func (stubOutput) Close() error { return nil }
