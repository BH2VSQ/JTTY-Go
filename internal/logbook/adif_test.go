package logbook

import (
	"strings"
	"testing"
	"time"
)

func TestFormatADIF(t *testing.T) {
	q := QSO{StartUTC: time.Date(2026, 9, 28, 12, 34, 56, 0, time.UTC), EndUTC: time.Date(2026, 9, 28, 12, 35, 10, 0, time.UTC), Call: "JA1ABC", Grid: "PM95", Name: "Taro", PowerWatts: 25, Operator: "BH2VSQ", Band: "20m", Frequency: 14090000, Mode: "JTTY"}
	got := FormatADIF(q)
	for _, want := range []string{"<QSO_DATE:8>20260928", "<TIME_ON:6>123456", "<CALL:6>JA1ABC", "<FREQ:9>14.090000", "<GRIDSQUARE:4>PM95", "<NAME:4>Taro", "<TX_PWR:4>25.0", "<OPERATOR:6>BH2VSQ", "<MODE:4>JTTY", "<EOR>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestFormatADIFIncludesStationGrid(t *testing.T) {
	q := QSO{
		StartUTC:        time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC),
		Call:            "K1ABC",
		Grid:            "FN42",
		StationCallsign: "N0CALL",
		StationGrid:     "PM95",
		Band:            "20m",
		Frequency:       14090000,
		Mode:            "JTTY",
		RSTSent:         "599",
		RSTRcvd:         "599",
	}
	got := FormatADIF(q)
	if !strings.Contains(got, "<MY_GRIDSQUARE:4>PM95") {
		t.Fatalf("ADIF missing MY_GRIDSQUARE: %s", got)
	}
	if !strings.Contains(got, "<GRIDSQUARE:4>FN42") {
		t.Fatalf("ADIF missing contacted station grid: %s", got)
	}
}
