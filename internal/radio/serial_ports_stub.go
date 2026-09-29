//go:build !windows

package radio

// AvailableSerialPorts is intentionally empty on non-Windows builds.
// JTTY's CAT serial auto-discovery is currently a Windows runtime feature.
func AvailableSerialPorts() []string { return nil }
