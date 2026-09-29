package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

type testSink struct {
	events []string
}

func (s *testSink) Emit(event string, payload any) {
	s.events = append(s.events, event)
}

func TestManualQSOStartRequiresActualDXInformationSend(t *testing.T) {
	a := New()
	a.SetDXCall("JA1ABC")

	t0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	a.noteQSOTransmit(t0, "CQ BH2VSQ CQ")
	if _, ok := a.qsoStarts["JA1ABC"]; ok {
		t.Fatal("QSO start created without the current DX callsign in the transmitted message")
	}

	a.noteQSOTransmit(t0.Add(time.Second), "JA1ABC 599 001")
	got, ok := a.qsoStarts["JA1ABC"]
	if !ok {
		t.Fatal("QSO start was not captured for an actual information send to the DX call")
	}
	if !got.Equal(t0.Add(time.Second)) {
		t.Fatalf("start=%v want %v", got, t0.Add(time.Second))
	}

	a.noteQSOTransmit(t0.Add(3*time.Second), "JA1ABC TU")
	got2 := a.qsoStarts["JA1ABC"]
	if !got2.Equal(got) {
		t.Fatalf("first send timestamp changed: got %v want %v", got2, got)
	}
}

func TestDefaultSettingsDisableAutomaticQSOLoggingRules(t *testing.T) {
	s := model.DefaultSettings()
	if len(s.Automation) != 0 {
		t.Fatalf("automatic QSO rules still enabled by default: %+v", s.Automation)
	}
}

func TestDefaultRadioUsesVOXWithoutCAT(t *testing.T) {
	s := model.DefaultSettings()
	if s.Radio.Backend != "none" {
		t.Fatalf("backend=%q want none", s.Radio.Backend)
	}
	if s.Radio.PTTMethod != "VOX" {
		t.Fatalf("ptt=%q want VOX", s.Radio.PTTMethod)
	}
	if s.AutoStartMonitor {
		t.Fatal("automatic monitor startup should be disabled by default")
	}
}

func TestLegacyRadioSettingsAreMigratedToVOX(t *testing.T) {
	s := model.DefaultSettings()
	s.Radio.Backend = "hamlib-rigctld"
	s.Radio.RigModelID = 0
	s.Radio.RigName = "None"
	s.Radio.PTTMethod = "CAT"
	out, changed := normalizeLoadedSettings(s, `C:\JTTY-Go\record`)
	if !changed {
		t.Fatal("legacy settings were not marked as migrated")
	}
	if out.Radio.Backend != "none" || out.Radio.PTTMethod != "VOX" {
		t.Fatalf("legacy radio settings not normalized: %+v", out.Radio)
	}
}

func TestSelectedRadioPreservesVOXPTTMethod(t *testing.T) {
	s := model.DefaultSettings()
	s.Radio.Backend = "hamlib-rigctld"
	s.Radio.RigModelID = 1020
	s.Radio.RigName = "Yaesu FT-817"
	s.Radio.PTTMethod = "VOX"
	s.Audio.RecordDirectory = `C:\JTTY-Go\record`
	out, changed := normalizeLoadedSettings(s, `C:\JTTY-Go\record`)
	if changed {
		t.Fatal("valid selected-radio VOX configuration should not be migrated")
	}
	if out.Radio.Backend != "hamlib-rigctld" || out.Radio.RigModelID != 1020 || out.Radio.PTTMethod != "VOX" {
		t.Fatalf("selected radio VOX settings changed unexpectedly: %+v", out.Radio)
	}
}

func TestTestPTTEligibilityMatchesWSJTXControlMethods(t *testing.T) {
	base := model.DefaultSettings().Radio
	base.RigModelID = 1020
	for _, method := range []string{"CAT", "DTR", "RTS"} {
		base.PTTMethod = method
		if !testPTTEligible(base) {
			t.Fatalf("PTT method %s should be independently testable", method)
		}
	}
	base.PTTMethod = "VOX"
	if testPTTEligible(base) {
		t.Fatal("VOX should not expose an independent PTT test")
	}
	base.RigModelID = 0
	base.PTTMethod = "CAT"
	if testPTTEligible(base) {
		t.Fatal("None must not expose an independent CAT PTT test")
	}
	for _, method := range []string{"DTR", "RTS"} {
		base.PTTMethod = method
		if !testPTTEligible(base) {
			t.Fatalf("None should allow direct serial %s PTT test", method)
		}
	}
}

func TestTestRadioConfigEqualityCoversDraftRadioChanges(t *testing.T) {
	a := model.DefaultSettings().Radio
	a.RigModelID = 1020
	a.PTTMethod = "CAT"
	b := a
	if !testRadioConfigEqual(a, b) {
		t.Fatal("identical test-rig configurations should compare equal")
	}
	b.Mode = "LSB"
	if testRadioConfigEqual(a, b) {
		t.Fatal("mode changes must invalidate the persistent test connection")
	}
	b = a
	b.PTTMethod = "DTR"
	if testRadioConfigEqual(a, b) {
		t.Fatal("PTT method changes must invalidate the persistent test connection")
	}
}

func TestBandDialFrequency(t *testing.T) {
	f, ok := bandDialFrequency("20")
	if !ok || f != 14090000 {
		t.Fatalf("20m=%d ok=%v", f, ok)
	}
	if _, ok := bandDialFrequency("999"); ok {
		t.Fatal("unexpected frequency for unknown band")
	}
}

func TestRecordQSOUsesDXGridFallback(t *testing.T) {
	a := New()
	a.configPath = filepath.Join(t.TempDir(), "config.json")
	a.SetDXCall("JA1ABC")
	a.SetDXGrid("PM95")
	start := time.Date(2026, 9, 28, 12, 1, 2, 0, time.UTC)
	a.qsoStarts["JA1ABC"] = start
	end := start.Add(time.Minute)
	payload := map[string]any{"call": "JA1ABC", "frequency": 14090000, "endUtc": end.Format(time.RFC3339Nano)}
	if err := a.recordQSOFromPayload(payload); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(a.configPath), "JTTY.adi"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<GRIDSQUARE:4>PM95") {
		t.Fatalf("DX grid not written: %s", string(data))
	}
}

func TestRecordQSOWritesManualUTCInterval(t *testing.T) {
	a := New()
	a.configPath = filepath.Join(t.TempDir(), "config.json")
	a.SetDXCall("JA1ABC")
	start := time.Date(2026, 9, 28, 12, 1, 2, 0, time.UTC)
	end := start.Add(47 * time.Second)
	a.qsoStarts["JA1ABC"] = start

	payload := map[string]any{
		"call":      "JA1ABC",
		"mode":      "JTTY",
		"band":      "20m",
		"frequency": 14090000,
		"endUtc":    end.Format(time.RFC3339Nano),
		"rstSent":   "599",
		"rstRcvd":   "599",
	}
	if err := a.recordQSOFromPayload(payload); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(a.configPath), "JTTY.adi"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"<QSO_DATE:8>20260928",
		"<TIME_ON:6>120102",
		"<QSO_DATE_OFF:8>20260928",
		"<TIME_OFF:6>120149",
		"<CALL:6>JA1ABC",
		"<RST_SENT:3>599",
		"<RST_RCVD:3>599",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if _, ok := a.qsoStarts["JA1ABC"]; ok {
		t.Fatal("QSO start should be consumed after successful manual logging")
	}
}
