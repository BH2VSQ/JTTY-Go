package app

import (
	"encoding/json"
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

func TestIndependentSerialPTTPersistsWithoutRigSelection(t *testing.T) {
	for _, method := range []string{"RTS", "DTR", "VOX"} {
		t.Run(method, func(t *testing.T) {
			a := New()
			a.configPath = filepath.Join(t.TempDir(), "config.json")
			s := a.Settings()
			s.Radio.Backend = "none"
			s.Radio.RigModelID = 0
			s.Radio.RigName = "None"
			s.Radio.PTTMethod = method
			s.Radio.PTTSerialPort = "COM77"

			if err := a.SaveSettings(s); err != nil {
				t.Fatalf("SaveSettings: %v", err)
			}
			if got := a.Settings().Radio.PTTMethod; got != method {
				t.Fatalf("runtime PTT method=%q, want %q", got, method)
			}
			data, err := os.ReadFile(a.configPath)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			var persisted model.Settings
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatalf("decode config: %v", err)
			}
			if persisted.Radio.PTTMethod != method {
				t.Fatalf("persisted PTT method=%q, want %q", persisted.Radio.PTTMethod, method)
			}
			loaded, _ := normalizeLoadedSettings(persisted, filepath.Join(t.TempDir(), "record"))
			if loaded.Radio.PTTMethod != method {
				t.Fatalf("reloaded PTT method=%q, want %q", loaded.Radio.PTTMethod, method)
			}
		})
	}
}

func TestDuplexModeSurvivesGenericSettingsSave(t *testing.T) {
	a := New()
	a.configPath = filepath.Join(t.TempDir(), "config.json")
	a.settingsMu.Lock()
	a.settings.DuplexMode = true
	stale := a.settings
	stale.DuplexMode = false
	a.settingsMu.Unlock()
	if err := a.SaveSettings(stale); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if !a.Settings().DuplexMode {
		t.Fatal("generic settings save cleared the live duplex mode")
	}
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var persisted model.Settings
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if !persisted.DuplexMode {
		t.Fatal("generic settings save cleared persisted duplex mode")
	}
}

func TestPTTOnlyChangesDoNotRequireRigReconnect(t *testing.T) {
	a := model.DefaultSettings().Radio
	a.RigModelID = 1020
	a.RigName = "Test Rig"
	b := a
	b.PTTMethod = "RTS"
	b.PTTSerialPort = "COM77"
	b.ForceDTR = "off"
	if !radioControlConfigEqual(a, b) {
		t.Fatal("PTT-only changes should not count as a rig-control change")
	}
	b.Host = "192.0.2.1"
	if radioControlConfigEqual(a, b) {
		t.Fatal("a rigctl host change must count as a rig-control change")
	}
}

func TestRadioPTTSettingsRemainIndependentAndPersistentV2(t *testing.T) {
	for _, method := range []string{"RTS", "DTR", "VOX"} {
		t.Run(method, func(t *testing.T) {
			a := New()
			a.configPath = filepath.Join(t.TempDir(), "config.json")
			s := a.Settings()
			s.Radio.RigModelID = 0
			s.Radio.RigName = "None"
			s.Radio.Backend = "none"
			s.Radio.PTTMethod = method
			s.Radio.PTTSerialPort = "COM77"
			if err := a.SaveSettings(s); err != nil {
				t.Fatalf("SaveSettings: %v", err)
			}
			if got := a.Settings().Radio.PTTMethod; got != method {
				t.Fatalf("runtime method=%q want %q", got, method)
			}
			data, err := os.ReadFile(a.configPath)
			if err != nil {
				t.Fatal(err)
			}
			var saved model.Settings
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Radio.PTTMethod != method {
				t.Fatalf("persisted method=%q want %q", saved.Radio.PTTMethod, method)
			}
			loaded, _ := normalizeLoadedSettings(saved, filepath.Join(t.TempDir(), "record"))
			if loaded.Radio.PTTMethod != method {
				t.Fatalf("reloaded method=%q want %q", loaded.Radio.PTTMethod, method)
			}
		})
	}
}

func TestNETRadioEnableAndPTTChoicePersistAcrossReloadV1(t *testing.T) {
	for _, backend := range []string{"rigctld-net", "none"} {
		for _, method := range []string{"VOX", "Rigctl", "RTS", "DTR"} {
			t.Run(backend+"/"+method, func(t *testing.T) {
				a := New()
				a.configPath = filepath.Join(t.TempDir(), "config.json")
				a.settingsMu.Lock()
				a.settings.Radio.Backend = backend
				a.settings.Radio.RigModelID = 0
				a.settings.Radio.RigName = "None"
				a.settings.Radio.Host = "192.0.2.10"
				a.settings.Radio.Port = 4532
				a.settings.Radio.PTTMethod = method
				a.settings.Radio.PTTSerialPort = "COM77"
				a.settingsMu.Unlock()

				s := a.Settings()
				if err := a.SaveSettings(s); err != nil {
					t.Fatalf("SaveSettings: %v", err)
				}
				if got := a.Settings().Radio.Backend; got != backend {
					t.Fatalf("runtime backend=%q want %q", got, backend)
				}
				if got := a.Settings().Radio.PTTMethod; got != method {
					t.Fatalf("runtime PTT method=%q want %q", got, method)
				}
				data, err := os.ReadFile(a.configPath)
				if err != nil {
					t.Fatalf("read persisted config: %v", err)
				}
				var persisted model.Settings
				if err := json.Unmarshal(data, &persisted); err != nil {
					t.Fatalf("decode persisted config: %v", err)
				}
				if got := persisted.Radio.Backend; got != backend {
					t.Fatalf("persisted backend=%q want %q", got, backend)
				}
				if got := persisted.Radio.PTTMethod; got != method {
					t.Fatalf("persisted PTT method=%q want %q", got, method)
				}
				if persisted.Radio.PTTSerialPort != "COM77" || persisted.Radio.Host != "192.0.2.10" || persisted.Radio.Port != 4532 {
					t.Fatalf("persisted NET/PTT fields changed unexpectedly: %+v", persisted.Radio)
				}
				loaded, _ := normalizeLoadedSettings(persisted, filepath.Join(t.TempDir(), "record"))
				if loaded.Radio.Backend != backend {
					t.Fatalf("reloaded backend=%q want %q", loaded.Radio.Backend, backend)
				}
				if loaded.Radio.PTTMethod != method {
					t.Fatalf("reloaded PTT method=%q want %q", loaded.Radio.PTTMethod, method)
				}
			})
		}
	}
}

func TestPTTChoiceDoesNotDisableNETRadioControlV1(t *testing.T) {
	base := model.DefaultSettings().Radio
	base.Backend = "rigctld-net"
	base.RigModelID = 0
	base.RigName = "None"
	base.Host = "192.0.2.30"
	base.Port = 4532
	base.PTTMethod = "Rigctl"
	for _, method := range []string{"Rigctl", "RTS", "DTR", "VOX"} {
		candidate := base
		candidate.PTTMethod = method
		if !sameRadioControlConfig(base, candidate) {
			t.Errorf("changing PTT method to %s must not change the NET radio-control configuration", method)
		}
		normalized, _ := normalizeLoadedSettings(model.Settings{Radio: candidate, Audio: model.AudioSettings{RecordDirectory: `C:\JTTY-Go\record`}}, `C:\JTTY-Go\record`)
		if normalized.Radio.Backend != "rigctld-net" {
			t.Errorf("PTT method %s changed NET backend to %q", method, normalized.Radio.Backend)
		}
		if normalized.Radio.PTTMethod != method {
			t.Errorf("PTT method changed after load normalization: got %q want %q", normalized.Radio.PTTMethod, method)
		}
	}
}

func TestRadioEnabledExplicitFlagPersistsAcrossPTTChoicesV2(t *testing.T) {
	for _, method := range []string{"Rigctl", "RTS", "DTR", "VOX"} {
		t.Run(method, func(t *testing.T) {
			a := New()
			a.configPath = filepath.Join(t.TempDir(), "config.json")
			radioCfg := model.DefaultSettings().Radio
			radioCfg.Enabled = true
			radioCfg.Backend = "rigctld-net"
			radioCfg.RigModelID = 0
			radioCfg.RigName = "None"
			radioCfg.Host = "192.0.2.25"
			radioCfg.Port = 4532
			radioCfg.PTTMethod = method
			radioCfg.PTTSerialPort = "COM77"
			a.settings.Radio = radioCfg

			settings := model.DefaultSettings()
			settings.Radio = radioCfg
			// Simulate a stale compatibility backend while the explicit enable
			// checkbox remains on. The explicit flag must win when saved/reloaded.
			settings.Radio.Backend = "none"
			settings.Audio.RecordDirectory = filepath.Join(t.TempDir(), "record")
			if err := a.SaveSettings(settings); err != nil {
				t.Fatalf("SaveSettings: %v", err)
			}

			data, err := os.ReadFile(a.configPath)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			var persisted model.Settings
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatalf("decode config: %v", err)
			}
			if !persisted.Radio.Enabled || persisted.Radio.Backend != "rigctld-net" {
				t.Fatalf("NET enabled state not persisted: enabled=%v backend=%q", persisted.Radio.Enabled, persisted.Radio.Backend)
			}
			if persisted.Radio.PTTMethod != method || persisted.Radio.PTTSerialPort != "COM77" {
				t.Fatalf("PTT settings not persisted: method=%q port=%q", persisted.Radio.PTTMethod, persisted.Radio.PTTSerialPort)
			}
			loaded, _ := normalizeLoadedSettings(persisted, filepath.Join(t.TempDir(), "record"))
			if !loaded.Radio.Enabled || loaded.Radio.Backend != "rigctld-net" {
				t.Fatalf("NET enable state lost on reload: enabled=%v backend=%q", loaded.Radio.Enabled, loaded.Radio.Backend)
			}
			if loaded.Radio.PTTMethod != method {
				t.Fatalf("PTT method changed on reload: got %q want %q", loaded.Radio.PTTMethod, method)
			}
		})
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
	if !changed {
		t.Fatal("legacy local-Hamlib settings should be marked for NET migration")
	}
	if out.Radio.Backend != "rigctld-net" || out.Radio.RigModelID != 1020 || out.Radio.PTTMethod != "VOX" {
		t.Fatalf("selected radio migration changed unexpected fields: %+v", out.Radio)
	}
}

func TestTestPTTEligibilityMatchesNetworkPTTMethods(t *testing.T) {
	base := model.DefaultSettings().Radio
	base.Host = "127.0.0.1"
	base.Port = 4532
	for _, method := range []string{"Rigctl", "CAT"} {
		base.PTTMethod = method
		if !testPTTEligible(base) {
			t.Fatalf("PTT method %s should be testable with a valid NET endpoint", method)
		}
	}
	base.PTTMethod = "Rigctl"
	base.Host = ""
	if testPTTEligible(base) {
		t.Fatal("Rigctl should require a server address")
	}
	base.Host = "127.0.0.1"
	base.Port = 70000
	if testPTTEligible(base) {
		t.Fatal("Rigctl should reject an invalid TCP port")
	}
	base.PTTMethod = "VOX"
	base.Port = 4532
	if testPTTEligible(base) {
		t.Fatal("VOX should not expose an independent PTT toggle")
	}
	base.RigModelID = 0
	for _, method := range []string{"DTR", "RTS"} {
		base.PTTMethod = method
		if !testPTTEligible(base) {
			t.Fatalf("direct local serial %s should be testable without a selected rig", method)
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
