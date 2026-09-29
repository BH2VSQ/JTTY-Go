//go:build windows

package radio

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var queryDosDeviceW = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryDosDeviceW")

// AvailableSerialPorts returns currently present Windows COM ports. It uses
// the same style of live enumeration as WSJT-X's QSerialPortInfo list, without
// adding a third-party Go serial dependency.
func AvailableSerialPorts() []string {
	ports := make([]string, 0, 16)
	for n := 1; n <= 256; n++ {
		name := fmt.Sprintf("COM%d", n)
		namePtr, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		buf := make([]uint16, 32768)
		r1, _, _ := queryDosDeviceW.Call(
			uintptr(unsafe.Pointer(namePtr)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(uint32(len(buf))),
		)
		if r1 != 0 {
			ports = append(ports, name)
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		ni := comNumber(ports[i])
		nj := comNumber(ports[j])
		if ni != nj {
			return ni < nj
		}
		return strings.Compare(ports[i], ports[j]) < 0
	})
	return ports
}

func comNumber(port string) int {
	port = strings.TrimSpace(strings.ToUpper(port))
	if !strings.HasPrefix(port, "COM") {
		return 1 << 30
	}
	n, err := strconv.Atoi(strings.TrimPrefix(port, "COM"))
	if err != nil {
		return 1 << 30
	}
	return n
}
