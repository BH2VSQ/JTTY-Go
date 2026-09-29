//go:build !windows

package radio

import "fmt"

type stubPTTSerial struct{}

func NewPTTSerial() PTTSerial { return stubPTTSerial{} }
func (stubPTTSerial) Set(port, method string, on bool) error {
	return fmt.Errorf("serial PTT is only available on Windows")
}
func (stubPTTSerial) Close() error { return nil }
