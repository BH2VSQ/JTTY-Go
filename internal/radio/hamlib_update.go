package radio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	cmd := exec.Command(exe, "-V")
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
		if version, probeErr := ProbeHamlibDLLVersion(dll); probeErr == nil {
			status.DLLVersion = version
			status.DLLVerified = true
		} else if status.Note == "" {
			status.Note = probeErr.Error()
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
		if version, err := ProbeHamlibVersion(exe, exeDir); err == nil {
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
	if runtime.GOOS != "windows" {
		return HamlibUpdateStatus{}, fmt.Errorf("Hamlib update only available on Windows")
	}
	if arch != "32" && arch != "64" {
		arch = "64"
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
	tmp := filepath.Join(dir, "libhamlib-4_new.dll")
	_ = os.Remove(tmp)

	url := hamlibURL(arch)
	setHamlibHeaders := func(req *http.Request) {
		req.Header.Set("User-Agent", "Downloading latest libhamlib-4.dll")
		req.Header.Set("Accept", "*/*")
	}

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return HamlibUpdateStatus{}, err
	}
	setHamlibHeaders(headReq)
	headResp, err := http.DefaultClient.Do(headReq)
	if err != nil {
		return HamlibUpdateStatus{}, fmt.Errorf("check Hamlib download: %w", err)
	}
	headResp.Body.Close()
	if headResp.StatusCode < 200 || headResp.StatusCode >= 300 {
		if headResp.StatusCode == http.StatusForbidden {
			return HamlibUpdateStatus{}, fmt.Errorf("check Hamlib download: HTTP 403 Forbidden")
		}
		return HamlibUpdateStatus{}, fmt.Errorf("check Hamlib download: HTTP %s", headResp.Status)
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HamlibUpdateStatus{}, err
	}
	setHamlibHeaders(getReq)
	resp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return HamlibUpdateStatus{}, fmt.Errorf("download Hamlib: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusForbidden {
			return HamlibUpdateStatus{}, fmt.Errorf("download Hamlib: HTTP 403 Forbidden")
		}
		return HamlibUpdateStatus{}, fmt.Errorf("download Hamlib: HTTP %s", resp.Status)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return HamlibUpdateStatus{}, err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return HamlibUpdateStatus{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return HamlibUpdateStatus{}, closeErr
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() < 1024*1024 {
		_ = os.Remove(tmp)
		return HamlibUpdateStatus{}, fmt.Errorf("downloaded Hamlib DLL is unexpectedly small")
	}
	if version, err := ProbeHamlibDLLVersion(tmp); err != nil {
		_ = os.Remove(tmp)
		return HamlibUpdateStatus{}, fmt.Errorf("downloaded Hamlib DLL cannot be loaded: %w", err)
	} else if strings.TrimSpace(version) == "" {
		_ = os.Remove(tmp)
		return HamlibUpdateStatus{}, fmt.Errorf("downloaded Hamlib DLL returned no version")
	}

	if fileExists(backup) {
		if err := removeWithRetry(backup); err != nil {
			_ = os.Remove(tmp)
			return HamlibUpdateStatus{}, fmt.Errorf("remove old Hamlib backup: %w", err)
		}
	}
	if fileExists(dll) {
		if err := renameWithRetry(dll, backup); err != nil {
			if isSharingViolation(err) {
				stopHamlibClients(ctx)
				err = renameWithRetry(dll, backup)
			}
			if err != nil {
				_ = os.Remove(tmp)
				return HamlibUpdateStatus{}, fmt.Errorf("backup current Hamlib: %w; the Hamlib DLL may still be in use by rigctld.exe or another process", err)
			}
		}
	}
	if err := renameWithRetry(tmp, dll); err != nil {
		if isSharingViolation(err) {
			stopHamlibClients(ctx)
			err = renameWithRetry(tmp, dll)
		}
		if err != nil {
			if fileExists(backup) && !fileExists(dll) {
				_ = renameWithRetry(backup, dll)
			}
			if isSharingViolation(err) {
				return HamlibUpdateStatus{}, fmt.Errorf("install new Hamlib: %w; the DLL is still in use, close rigctld.exe/other Hamlib programs and retry", err)
			}
			return HamlibUpdateStatus{}, fmt.Errorf("install new Hamlib: %w", err)
		}
	}

	status, statusErr := HamlibStatusWithExecutable(arch, dir, "")
	if statusErr != nil {
		return HamlibUpdateStatus{}, statusErr
	}
	return status, nil
}

func RevertHamlib(arch, installDir string) (HamlibUpdateStatus, error) {
	if runtime.GOOS != "windows" {
		return HamlibUpdateStatus{}, fmt.Errorf("Hamlib update only available on Windows")
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
	staged := filepath.Join(dir, "libhamlib-4_new.dll")
	if !fileExists(backup) {
		return HamlibStatus(arch, dir)
	}
	_ = os.Remove(staged)
	if fileExists(dll) {
		if err := renameWithRetry(dll, staged); err != nil {
			if isSharingViolation(err) {
				stopHamlibClients(context.Background())
				err = renameWithRetry(dll, staged)
			}
			if err != nil {
				return HamlibUpdateStatus{}, fmt.Errorf("stage current Hamlib: %w; the DLL may still be in use by rigctld.exe or another process", err)
			}
		}
	}
	if err := renameWithRetry(backup, dll); err != nil {
		if fileExists(staged) {
			_ = renameWithRetry(staged, dll)
		}
		return HamlibUpdateStatus{}, err
	}
	return HamlibStatus(arch, dir)
}

func renameWithRetry(oldPath, newPath string) error {
	var last error
	for i := 0; i < 20; i++ {
		if err := os.Rename(oldPath, newPath); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	return last
}

func isSharingViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "being used by another process") ||
		strings.Contains(msg, "process cannot access the file") ||
		strings.Contains(msg, "sharing violation")
}

func stopHamlibClients(ctx context.Context) {
	if runtime.GOOS != "windows" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, image := range []string{"rigctld.exe", "rigctl.exe"} {
		cmd := exec.CommandContext(ctx, "taskkill", "/F", "/T", "/IM", image)
		_ = cmd.Run()
	}
	time.Sleep(300 * time.Millisecond)
}

func removeWithRetry(path string) error {
	var last error
	for i := 0; i < 20; i++ {
		err := os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		last = err
		time.Sleep(150 * time.Millisecond)
	}
	return last
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
