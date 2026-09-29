package model

import "testing"

func TestDefaultFrequencyBands(t *testing.T) {
	want := []struct {
		band string
		hz   int64
	}{
		{"160m", 1838000},
		{"80m", 3575000},
		{"60m", 5357000},
		{"40m", 7090000},
		{"30m", 10140000},
		{"20m", 14090000},
		{"17m", 18100000},
		{"15m", 21090000},
		{"12m", 24920000},
		{"10m", 28090000},
		{"6m", 50316000},
		{"2m", 144077000},
		{"70cm", 432077000},
	}
	got := DefaultFrequencyBands()
	if len(got) != len(want) {
		t.Fatalf("got %d bands, want %d", len(got), len(want))
	}
	for i, band := range got {
		if band.Band != want[i].band {
			t.Errorf("band %d: got %q, want %q", i, band.Band, want[i].band)
		}
		if band.DefaultHz != want[i].hz {
			t.Errorf("band %s: got default %d Hz, want %d Hz", band.Band, band.DefaultHz, want[i].hz)
		}
		if len(band.Frequencies) != 1 || band.Frequencies[0] != want[i].hz {
			t.Errorf("band %s: got frequency list %#v, want [%d]", band.Band, band.Frequencies, want[i].hz)
		}
	}
}

func TestDefaultSettingsPersistedStateDefaults(t *testing.T) {
	s := DefaultSettings()
	if s.RXFrequencyHz != 1500 || s.TXFrequencyHz != 1500 {
		t.Fatalf("unexpected default RX/TX frequencies: %d/%d", s.RXFrequencyHz, s.TXFrequencyHz)
	}
	if s.Layout.WindowWidth != 1360 || s.Layout.WindowHeight != 820 {
		t.Fatalf("unexpected default window size: %dx%d", s.Layout.WindowWidth, s.Layout.WindowHeight)
	}
}
