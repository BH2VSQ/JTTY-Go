package radio

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func bundledRuntime(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("bundled runtime targets Windows amd64")
	}
	if dir := os.Getenv("JTTY_HAMLIB_DIR"); dir != "" {
		return filepath.Join(dir, "rigctld.exe")
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "bin", "rigctld.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestManagedRealHamlibDummyAndExternalNET(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	managed, err := StartManaged(ctx, bundledRuntime(t), []string{"-m", "1", "-P", "RIG"}, RigctldConfig{Mode: "USB", Port: occupied.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	defer managed.Close()
	if managed.cfg.Port == occupied.Addr().(*net.TCPAddr).Port {
		t.Fatal("local child reused occupied NET endpoint")
	}
	caps, err := managed.Capabilities(ctx)
	if err != nil || !caps.HasGetFreq || !caps.HasSetFreq || !caps.HasSetPTT {
		t.Fatalf("caps=%+v err=%v", caps, err)
	}
	if err = managed.SetFrequency(ctx, 14091000); err != nil {
		t.Fatal(err)
	}
	if hz, err := managed.Frequency(ctx); err != nil || hz != 14091000 {
		t.Fatalf("frequency=%d %v", hz, err)
	}
	if err = managed.SetMode(ctx, "USB"); err != nil {
		t.Fatal(err)
	}
	if err = managed.PTT(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !managed.Transmitting() {
		t.Fatal("PTT did not turn on")
	}
	if err = managed.PTT(ctx, false); err != nil {
		t.Fatal(err)
	}
	_, _, err = managed.Split(ctx)
	if err != nil {
		t.Fatal(err)
	}
	vfo := caps.SupportedVFOs[1]
	if err = managed.SetSplitEnabled(ctx, true, vfo); err != nil {
		t.Fatal(err)
	}
	if err = managed.SetSplitFrequency(ctx, 14092000); err != nil {
		t.Fatal(err)
	}
	if err = managed.SetSplitEnabled(ctx, false, vfo); err != nil {
		t.Fatal(err)
	}
	client := NewRigctld(managed.cfg)
	if err = client.Open(ctx); err != nil {
		t.Fatal(err)
	}
	if err = client.Close(); err != nil {
		t.Fatal(err)
	}
	if !managed.Running() {
		t.Fatal("NET client stopped externally owned server")
	}
	if err = managed.Close(); err != nil {
		t.Fatal(err)
	}
	if managed.Running() {
		t.Fatal("owned child survived Close")
	}
}

func TestManagedStartupFailureIncludesDiagnostics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := StartManaged(ctx, bundledRuntime(t), []string{"-m", "99999999"}, RigctldConfig{})
	if r != nil || err == nil {
		if r != nil {
			r.Close()
		}
		t.Fatal("invalid model unexpectedly started")
	}
}
