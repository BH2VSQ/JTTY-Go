//go:build windows

package audio

import (
	"fmt"

	"github.com/go-ole/go-ole"
)

// initializeCOM initializes COM on the current OS thread. go-ole reports
// HRESULT S_FALSE (0x00000001) as an *OleError because CoInitializeEx returns
// an error value for that HRESULT. S_FALSE is still a successful initialization
// and must therefore be accepted.
func initializeCOM() error {
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err == nil {
		return nil
	}

	if oe, ok := err.(*ole.OleError); ok && oe.Code() == uintptr(1) {
		return nil
	}

	return fmt.Errorf("CoInitializeEx: %w", err)
}
