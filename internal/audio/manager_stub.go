//go:build !windows

package audio

import (
	"context"
	"fmt"
)

type stubManager struct{}

func NewManager() Manager { return stubManager{} }
func (stubManager) Inputs() ([]Device, error) {
	return nil, fmt.Errorf("Windows Core Audio backend is only available on Windows")
}
func (stubManager) Outputs() ([]Device, error) {
	return nil, fmt.Errorf("Windows Core Audio backend is only available on Windows")
}
func (stubManager) NewCapture() Source { return stubSource{} }
func (stubManager) NewOutput() Output  { return stubOutput{} }

type stubSource struct{}

func (stubSource) Open(ctx context.Context, cfg CaptureConfig, sink FrameSink) error {
	return fmt.Errorf("WASAPI capture is only available on Windows")
}
func (stubSource) Start(ctx context.Context) error {
	return fmt.Errorf("WASAPI capture is only available on Windows")
}
func (stubSource) Stop() error  { return nil }
func (stubSource) Close() error { return nil }
