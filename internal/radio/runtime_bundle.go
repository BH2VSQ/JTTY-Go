package radio

import (
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type RuntimeManifest struct {
	Version string            `json:"version"`
	Source  string            `json:"source"`
	Files   map[string]string `json:"files"`
}

// VerifyRuntime rejects partial upgrades and mixed architectures before a
// bundled executable is started. Custom packages must provide the same manifest.
func VerifyRuntime(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "runtime-manifest.json"))
	if err != nil {
		return fmt.Errorf("读取 Hamlib 运行包清单: %w", err)
	}
	var manifest RuntimeManifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	required := []string{"rigctld.exe", "rigctl.exe", "libhamlib-4.dll", "libgcc_s_seh-1.dll", "libwinpthread-1.dll", "libusb-1.0.dll"}
	for _, name := range required {
		want := manifest.Files[name]
		if len(want) != 64 {
			return fmt.Errorf("Hamlib 清单缺少 %s", name)
		}
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != want {
			return fmt.Errorf("Hamlib 文件校验失败: %s", name)
		}
		image, err := pe.Open(path)
		if err != nil {
			return err
		}
		machine := image.Machine
		_ = image.Close()
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return fmt.Errorf("Hamlib %s 不是 Windows amd64", name)
		}
	}
	return nil
}
