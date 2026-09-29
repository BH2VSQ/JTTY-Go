package radio

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstallEmbeddedHamlib(t *testing.T) {
	src := fstest.MapFS{
		"bin/rigctld.exe":     &fstest.MapFile{Data: []byte("rigctld")},
		"bin/libhamlib-4.dll": &fstest.MapFile{Data: []byte("hamlib")},
		"bin/.gitkeep":        &fstest.MapFile{Data: []byte{}},
	}
	target := t.TempDir()
	if err := InstallEmbeddedHamlib(fs.FS(src), target); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rigctld.exe", "libhamlib-4.dll"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}
