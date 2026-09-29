//go:build !windows

package radio

import "fmt"

func ProbeHamlibDLLVersion(dllPath string) (string, error) {
	return "", fmt.Errorf("Hamlib DLL probe is only available on Windows")
}
