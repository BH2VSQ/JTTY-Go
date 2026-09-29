//go:build windows

package audio

import (
	"fmt"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW = kernel32.NewProc("CreateEventW")
)

// createAudioEvent uses the native CreateEventW instead of go-wca's
// CreateEventExA wrapper. go-wca v0.4.1 exposes CreateEventExA as an error-only
// function, so the HANDLE returned by the Win32 API cannot be obtained through
// that wrapper.
func createAudioEvent() (uintptr, error) {
	// CreateEventW(NULL, FALSE, FALSE, NULL)
	handle, _, callErr := procCreateEventW.Call(0, 0, 0, 0)
	if handle != 0 {
		return handle, nil
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return 0, callErr
	}
	return 0, fmt.Errorf("CreateEventW failed")
}
