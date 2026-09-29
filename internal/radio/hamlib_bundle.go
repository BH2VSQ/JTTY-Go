package radio

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InstallEmbeddedHamlib releases the files stored below the embedded `bin/`
// tree into the application's runtime bin directory. Existing files are kept
// so a user-installed Hamlib update is not overwritten on every startup.
// Missing bundled files are materialized automatically.
func InstallEmbeddedHamlib(source fs.FS, targetDir string) error {
	if source == nil {
		return nil
	}
	if strings.TrimSpace(targetDir) == "" {
		return fmt.Errorf("Hamlib runtime directory is empty")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	found := false
	err := fs.WalkDir(source, "bin", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		// Ignore VCS/placeholder files. Real Hamlib binaries and supporting
		// DLLs under the package's bin directory are copied verbatim.
		nameLower := strings.ToLower(d.Name())
		if strings.HasPrefix(d.Name(), ".") || strings.HasSuffix(nameLower, ".txt") || strings.HasSuffix(nameLower, ".md") {
			return nil
		}
		found = true
		rel, err := filepath.Rel("bin", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)
		if fi, err := os.Stat(dest); err == nil && fi.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		in, err := source.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		tmp := dest + ".jtty.tmp"
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return nil
}
