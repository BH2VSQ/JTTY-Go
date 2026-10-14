package radio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type HamlibUpdateStatus struct {
	Architecture    string `json:"architecture"`
	DLLPath         string `json:"dllPath"`
	BackupPath      string `json:"backupPath"`
	DLLVersion      string `json:"dllVersion,omitempty"`
	DLLVerified     bool   `json:"dllVerified"`
	RuntimeVersion  string `json:"runtimeVersion,omitempty"`
	RuntimeVerified bool   `json:"runtimeVerified"`
	ExecutablePath  string `json:"executablePath,omitempty"`
	BackupExists    bool   `json:"backupExists"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
	SourceURL       string `json:"sourceUrl"`
	Note            string `json:"note,omitempty"`
}

const hamlibSnapshotBase = "https://hamlib.sourceforge.net/snapshots-4.7/dll"

func hamlibURL(arch string) string {
	if arch == "32" {
		return hamlibSnapshotBase + "32/libhamlib-4.dll"
	}
	return hamlibSnapshotBase + "64/libhamlib-4.dll"
}

// resolveHamlibExecutable accepts either a concrete rigctld/rigctl executable
// or a directory containing one. This avoids silently probing a different
// Hamlib installation when the settings field points at a directory.
func resolveHamlibExecutable(executable string) string {
	exe := strings.TrimSpace(executable)
	if exe != "" {
		if fi, err := os.Stat(exe); err == nil && fi.IsDir() {
			for _, name := range []string{"rigctld.exe", "rigctl.exe", "rigctld", "rigctl"} {
				candidate := filepath.Join(exe, name)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
		if fi, err := os.Stat(exe); err == nil && !fi.IsDir() {
			return exe
		}
		if path, err := exec.LookPath(exe); err == nil {
			return path
		}
	}
	for _, candidate := range []string{"rigctld.exe", "rigctl.exe", "rigctld", "rigctl"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

// ProbeHamlibVersion executes the local rigctld/rigctl using the executable's
// own directory as both its working directory and the first PATH entry. This
// makes the DLL selection deterministic on Windows.
func ProbeHamlibVersion(executable, installDir string) (string, error) {
	return ProbeHamlibVersionContext(context.Background(), executable, installDir)
}

// ProbeHamlibVersionContext is the cancellable variant used by the GUI. The
// settings page must be able to cancel a pending probe when it is closed.
func ProbeHamlibVersionContext(ctx context.Context, executable, installDir string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("Hamlib runtime probe is only available on Windows")
	}
	exe := resolveHamlibExecutable(executable)
	if exe == "" {
		return "", fmt.Errorf("未找到 rigctld.exe/rigctl.exe")
	}
	if installDir == "" {
		installDir = filepath.Dir(exe)
	}
	cmd := newHamlibCommandContext(ctx, exe, "-V")
	cmd.Dir = installDir
	cmd.Env = prependPath(cmd.Environ(), installDir)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return "", fmt.Errorf("Hamlib runtime probe: %w: %s", err, text)
		}
		return "", fmt.Errorf("Hamlib runtime probe: %w", err)
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("Hamlib runtime probe returned no version")
}

func prependPath(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	pathValue := dir
	found := false
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			pathValue += string(os.PathListSeparator) + item[5:]
			found = true
			break
		}
	}
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			out = append(out, "PATH="+pathValue)
		} else {
			out = append(out, item)
		}
	}
	if !found {
		out = append(out, "PATH="+pathValue)
	}
	return out
}

func HamlibStatus(arch, installDir string) (HamlibUpdateStatus, error) {
	return HamlibStatusWithExecutable(arch, installDir, "")
}

func hamlibVersionNumber(value string) string {
	for _, field := range strings.Fields(value) {
		if field == "" {
			continue
		}
		digitCount := 0
		dotCount := 0
		valid := true
		for _, r := range field {
			if r >= '0' && r <= '9' {
				digitCount++
			} else if r == '.' {
				dotCount++
			} else {
				valid = false
				break
			}
		}
		if valid && digitCount > 0 && dotCount >= 1 {
			return field
		}
	}
	return ""
}

func HamlibStatusWithExecutable(arch, installDir, executable string) (HamlibUpdateStatus, error) {
	return HamlibStatusWithExecutableContext(context.Background(), arch, installDir, executable)
}

// HamlibStatusWithExecutableContext is the cancellable GUI-facing status probe.
func HamlibStatusWithExecutableContext(ctx context.Context, arch, installDir, executable string) (HamlibUpdateStatus, error) {
	if runtime.GOOS != "windows" {
		return HamlibUpdateStatus{Architecture: arch, SourceURL: hamlibURL(arch), Note: "Hamlib DLL update is only available on Windows."}, nil
	}
	dir := installDir
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return HamlibUpdateStatus{}, err
		}
		dir = filepath.Dir(exe)
	}
	dir, _ = filepath.Abs(dir)
	dll := filepath.Join(dir, "libhamlib-4.dll")
	backup := filepath.Join(dir, "libhamlib-4_old.dll")
	status := HamlibUpdateStatus{Architecture: arch, DLLPath: dll, BackupPath: backup, BackupExists: fileExists(backup), SourceURL: hamlibURL(arch)}
	if fi, err := os.Stat(dll); err == nil {
		status.UpdatedAt = fi.ModTime().UTC().Format(time.RFC3339)
		// Version is read from the verified helper process. Loading a DLL into
		// the GUI just to inspect a C pointer is neither necessary nor safe.
		if err := VerifyRuntime(dir); err != nil {
			status.Note = err.Error()
		}
	} else if status.Note == "" {
		status.Note = "未找到 libhamlib-4.dll"
	}
	if !status.BackupExists && status.Note == "" {
		status.Note = "No Hamlib backup available"
	}

	exe := resolveHamlibExecutable(executable)
	if exe != "" {
		status.ExecutablePath = exe
		exeDir := filepath.Dir(exe)
		if version, err := ProbeHamlibVersionContext(ctx, exe, exeDir); err == nil {
			status.RuntimeVersion = version
			status.RuntimeVerified = true
			if status.DLLVerified {
				dllVersion := hamlibVersionNumber(status.DLLVersion)
				runtimeVersion := hamlibVersionNumber(status.RuntimeVersion)
				if dllVersion != "" && runtimeVersion != "" && dllVersion != runtimeVersion {
					status.Note = fmt.Sprintf("rigctld 实际加载的 Hamlib 与当前 DLL 不一致：DLL=%s；运行时=%s；请确认 libhamlib-4.dll 与 rigctld.exe 位于同一目录", status.DLLVersion, status.RuntimeVersion)
				}
			}
		} else if status.Note == "" {
			status.Note = err.Error()
		}
	} else if status.Note == "" {
		status.Note = "未找到 rigctld.exe/rigctl.exe，无法验证运行时 Hamlib"
	}
	return status, nil
}

func UpdateHamlib(ctx context.Context, arch, installDir string) (HamlibUpdateStatus, error) {
	return HamlibUpdateStatus{}, fmt.Errorf("Hamlib 随完整应用包升级，禁止单 DLL 替换")
}
func RevertHamlib(arch, installDir string) (HamlibUpdateStatus, error) {
	return HamlibUpdateStatus{}, fmt.Errorf("请安装完整的历史应用包以回滚 Hamlib")
}

func SaveHamlibStatus(path string, status HamlibUpdateStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
