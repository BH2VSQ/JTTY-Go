//go:build windows

package radio

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ProbeHamlibDLLVersion loads a Hamlib DLL directly and reads the exported
// hamlib_version2 variable. WSJT-X uses the same exported symbol to display
// the Hamlib version that is linked into the running program.
func ProbeHamlibDLLVersion(dllPath string) (string, error) {
	if dllPath == "" {
		return "", fmt.Errorf("Hamlib DLL path is empty")
	}
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return "", fmt.Errorf("加载 Hamlib DLL 失败: %w", err)
	}
	defer dll.Release()

	proc, err := dll.FindProc("hamlib_version2")
	if err != nil {
		return "", fmt.Errorf("Hamlib DLL 未导出 hamlib_version2: %w", err)
	}
	addr := proc.Addr()
	if addr == 0 {
		return "", fmt.Errorf("Hamlib hamlib_version2 地址无效")
	}

	versionPtr := *(*uintptr)(unsafe.Pointer(addr))
	if versionPtr == 0 {
		return "", fmt.Errorf("Hamlib hamlib_version2 指针为空")
	}

	const maxLen = 512
	raw := unsafe.Slice((*byte)(unsafe.Pointer(versionPtr)), maxLen)
	end := 0
	for end < len(raw) && raw[end] != 0 {
		end++
	}
	if end == 0 {
		return "", fmt.Errorf("Hamlib hamlib_version2 为空")
	}
	return string(raw[:end]), nil
}
