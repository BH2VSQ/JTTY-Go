package detector

import "testing"

func TestScannerZeroStartRange(t *testing.T) {
	s := NewScanner(ScannerConfig{SampleRate: 48000, MinFrequencyHz: 0, MaxFrequencyHz: 4000, FFTSize: 4096, ThresholdDb: 8})
	lo, hi := s.FrequencyRange()
	if lo != 0 || hi != 4000 {
		t.Fatalf("range = %v..%v, want 0..4000", lo, hi)
	}
	s.SetFrequencyRange(0, 2500)
	lo, hi = s.FrequencyRange()
	if lo != 0 || hi != 2500 {
		t.Fatalf("after update range = %v..%v, want 0..2500", lo, hi)
	}
}
