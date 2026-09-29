package logbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatDecodeLogLine(t *testing.T) {
	ts := time.Date(2026, 9, 27, 4, 45, 27, 0, time.UTC)
	got := FormatDecodeLogLine(ts, 50_316_000, 0, 0, 1644, "DE BG5JSU TNX SAT OM HPE CU AGN 73")
	want := "260927_044527    50.316 Rx JTTY     0  0.0 1644 DE BG5JSU TNX SAT OM HPE CU AGN 73\n"
	if got != want {
		t.Fatalf("unexpected line: %q", got)
	}
	if got := FormatDecodeLogLine(ts, 50_316_000, -7, 0.4, 1500, "DE  TEST\n"); !strings.Contains(got, " DE TEST\n") {
		t.Fatalf("log message spacing differs from displayed form: %q", got)
	}
}

func TestDecodeLogPath(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 9, 27, 4, 45, 27, 0, time.UTC)
	if got := filepath.Base(DecodeLogPath(dir, DecodeLogSingle, ts)); got != "ALL.txt" {
		t.Fatal(got)
	}
	if got := filepath.Base(DecodeLogPath(dir, DecodeLogYear, ts)); got != "ALL-2026.txt" {
		t.Fatal(got)
	}
	if got := filepath.Base(DecodeLogPath(dir, DecodeLogMonth, ts)); got != "ALL-2026-09.txt" {
		t.Fatal(got)
	}
}

func TestFormatTXLogLine(t *testing.T) {
	ts := time.Date(2026, 9, 27, 4, 45, 27, 0, time.UTC)
	want := "260927_044527    144.077 Tx JTTY     0  0.0 1700 CQ TEST PM95\n"
	if got := FormatTXLogLine(ts, 144_077_000, 1700, "CQ  TEST\nPM95"); got != want {
		t.Fatalf("unexpected TX line: %q", got)
	}
}

func TestDecodeLoggerAppendTX(t *testing.T) {
	dir := t.TempDir()
	l := NewDecodeLogger()
	ts := time.Date(2026, 9, 27, 4, 45, 27, 0, time.UTC)
	if err := l.AppendTX(dir, DecodeLogSingle, ts, 144_077_000, 1700, "CQ TEST PM95"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ALL.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Tx JTTY") || !strings.Contains(string(b), "1700 CQ TEST PM95") {
		t.Fatalf("TX line missing from ALL.txt: %q", string(b))
	}
	if _, err := os.Stat(filepath.Join(dir, "all.log")); !os.IsNotExist(err) {
		t.Fatalf("all.log should not be created, stat error=%v", err)
	}
}

func TestDecodeLoggerAppend(t *testing.T) {
	dir := t.TempDir()
	l := NewDecodeLogger()
	ts := time.Date(2026, 9, 27, 4, 45, 27, 0, time.UTC)
	if err := l.Append(dir, DecodeLogSingle, ts, 50_316_000, 0, 0, 1644, "DE TEST"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ALL.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "260927_044527") {
		t.Fatal(string(b))
	}
	if _, err := os.Stat(filepath.Join(dir, "all.log")); !os.IsNotExist(err) {
		t.Fatalf("all.log should not be created, stat error=%v", err)
	}
	if err := l.Append(dir, DecodeLogMonth, ts, 50_316_000, -7, 0.4, 1500, "DE  TEST"); err != nil {
		t.Fatal(err)
	}
	monthly, err := os.ReadFile(filepath.Join(dir, "ALL-2026-09.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(monthly), "DE TEST") {
		t.Fatalf("monthly log missing appended line: %q", string(monthly))
	}
	if _, err := os.Stat(filepath.Join(dir, "all.log")); !os.IsNotExist(err) {
		t.Fatalf("all.log should not be created in split mode, stat error=%v", err)
	}
}
