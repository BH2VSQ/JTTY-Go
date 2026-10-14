package app

import (
	"context"
	"github.com/BH2VSQ/jtty-go/internal/model"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLocalHamlibAppLifecycle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("bundled Windows runtime")
	}
	executable, _ := filepath.Abs(filepath.Join("..", "..", "bin", "rigctld.exe"))
	if dir := os.Getenv("JTTY_HAMLIB_DIR"); dir != "" {
		executable = filepath.Join(dir, "rigctld.exe")
	}
	a := New()
	cfg := a.settings.Radio
	cfg.Backend = "hamlib-local"
	cfg.RigModelID = 1
	cfg.RigctldPath = executable
	cfg.Mode = "USB"
	cfg.PTTMethod = "Rigctl"
	cfg.SplitMode = "None"
	a.settings.Radio = model.NormalizeRadio(cfg)
	if err := a.ConnectRadio(); err != nil {
		t.Fatal(err)
	}
	defer a.DisconnectRadio()
	if err := a.SetDialFrequency(14091000); err != nil {
		t.Fatal(err)
	}
	if err := a.setPTT(true); err != nil {
		t.Fatal(err)
	}
	if err := a.setPTT(false); err != nil {
		t.Fatal(err)
	}
	if err := a.DisconnectRadio(); err != nil {
		t.Fatal(err)
	}
	result := a.TestRadioSettings(a.settings)
	if !boolValue(result["ok"]) {
		t.Fatalf("test connection: %+v", result)
	}
	if a.testRadio.Transmitting() {
		t.Fatal("connection test keyed radio")
	}
	if err := a.testRadio.SetFrequency(context.Background(), 14092000); err != nil {
		t.Fatal(err)
	}
	a.closeTestRadio(false)
	if err := a.ConnectRadio(); err != nil {
		t.Fatal(err)
	}
	if result = a.TestRadioSettings(a.settings); !boolValue(result["ok"]) {
		t.Fatal(result)
	}
	// Switching draft backends closes tests without losing the original live session.
	if !a.closeTestRadio(false) {
		t.Fatal("lost restore marker")
	}
	a.closeTestRadio(true)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.radioMu.Lock()
		restored := a.radio != nil
		a.radioMu.Unlock()
		if restored {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cancel did not restore the live connection")
}
