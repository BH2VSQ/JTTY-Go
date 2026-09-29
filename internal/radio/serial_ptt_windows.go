//go:build windows

package radio

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	setDTR             = 5
	clrDTR             = 6
	setRTS             = 3
	clrRTS             = 4
	openExisting       = 3
	genericRead        = 0x80000000
	genericWrite       = 0x40000000
	fileShareRead      = 1
	fileShareWrite     = 2
	invalidHandleValue = ^uintptr(0)
)

var (
	k32         = syscall.NewLazyDLL("kernel32.dll")
	createFileW = k32.NewProc("CreateFileW")
	closeHandle = k32.NewProc("CloseHandle")
	escapeComm  = k32.NewProc("EscapeCommFunction")
)

type windowsPTTSerial struct{ h uintptr }

func NewPTTSerial() PTTSerial { return &windowsPTTSerial{} }
func (p *windowsPTTSerial) open(port string) error {
	if p.h != 0 {
		return nil
	}
	port = normalizeCOMPort(port)
	if port == "" {
		return fmt.Errorf("PTT 串口为空")
	}
	name, _ := syscall.UTF16PtrFromString(port)
	r, _, e := createFileW.Call(uintptr(unsafe.Pointer(name)), genericRead|genericWrite, fileShareRead|fileShareWrite, 0, openExisting, 0, 0)
	if r == invalidHandleValue {
		return fmt.Errorf("打开 PTT 串口 %s: %v", port, e)
	}
	p.h = r
	return nil
}
func (p *windowsPTTSerial) Set(port, method string, on bool) error {
	if err := p.open(port); err != nil {
		return err
	}
	var f uintptr
	switch strings.ToUpper(method) {
	case "DTR":
		if on {
			f = setDTR
		} else {
			f = clrDTR
		}
	case "RTS":
		if on {
			f = setRTS
		} else {
			f = clrRTS
		}
	default:
		return fmt.Errorf("不支持的 PTT 控制线: %s", method)
	}
	r, _, e := escapeComm.Call(p.h, f)
	if r == 0 {
		return fmt.Errorf("设置 %s: %v", method, e)
	}
	return nil
}
func (p *windowsPTTSerial) Close() error {
	if p.h == 0 {
		return nil
	}
	r, _, e := closeHandle.Call(p.h)
	p.h = 0
	if r == 0 {
		return e
	}
	return nil
}
func normalizeCOMPort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ""
	}
	if strings.HasPrefix(port, `\\.\`) {
		return port
	}
	return `\\.` + `\` + port
}
