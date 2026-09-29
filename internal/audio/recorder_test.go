package audio

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderSplitsAtUTCMidnight(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder()
	if err := r.Start(dir); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	r0 := time.Date(2026, 9, 28, 23, 59, 59, 900_000_000, time.UTC)
	r1 := time.Date(2026, 9, 29, 0, 0, 0, 100_000_000, time.UTC)
	frame0 := PCMFrame{Timestamp: r0, Samples: []float32{0, 0.5, -0.5}, SampleRate: 48000, Channels: 1}
	frame1 := PCMFrame{Timestamp: r1, Samples: []float32{0.25}, SampleRate: 48000, Channels: 1}
	if err := r.Write(frame0); err != nil {
		t.Fatal(err)
	}
	if err := r.Write(frame1); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260928.wav", "20260929.wav"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || string(data[36:40]) != "data" {
			t.Fatalf("bad wav %s", name)
		}
	}
}
