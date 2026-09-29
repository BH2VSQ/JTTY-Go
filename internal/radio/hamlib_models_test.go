package radio

import (
	"fmt"
	"testing"
)

func TestParseHamlibModelList(t *testing.T) {
	line := func(id int, mfg, model, version, status, macro string) string {
		// Match the fixed-column layout emitted by Hamlib's rigctl -l table.
		return fmt.Sprintf("%6d  %-22s %-23s %-15s %-11s %s", id, mfg, model, version, status, macro)
	}
	input := " Rig #  Mfg                    Model                   Version         Status      Macro\n" +
		line(1, "Hamlib", "Dummy", "20240709.0", "Stable", "RIG_MODEL_DUMMY") + "\n" +
		line(10, "N2ADR James Ahlstrom", "Quisk", "20230709.0", "Stable", "RIG_MODEL_QUISK") + "\n" +
		line(101, "Yaesu", "FT-891", "1.0", "Stable", "RIG_MODEL_FT_891") + "\n" +
		line(1020, "Yaesu", "FT-817", "20241117.0", "Stable", "RIG_MODEL_FT817") + "\n" +
		line(2029, "Elecraft", "K3", "2.0", "Beta", "RIG_MODEL_K3") + "\n" +
		line(3000, "Kenwood", "TS-2000X", "2.1", "Stable", "RIG_MODEL_TS2000") + "\n"

	got := ParseHamlibModelList(input)
	if len(got) != 6 {
		t.Fatalf("expected 6 models, got %d: %#v", len(got), got)
	}
	if got[0].Label != "Elecraft K3" || got[0].Manufacturer != "Elecraft" || got[0].Model != "K3" {
		t.Fatalf("expected alphabetic first row, got %#v", got[0])
	}
	var yaesu HamlibModel
	var quisk HamlibModel
	for _, m := range got {
		switch m.ModelID {
		case 1020:
			yaesu = m
		case 10:
			quisk = m
		}
	}
	if yaesu.Label != "Yaesu FT-817" || yaesu.Manufacturer != "Yaesu" || yaesu.Model != "FT-817" {
		t.Fatalf("unexpected FT-817 parsed row: %#v", yaesu)
	}
	if quisk.Manufacturer != "N2ADR James Ahlstrom" || quisk.Model != "Quisk" || quisk.Label != "N2ADR James Ahlstrom Quisk" {
		t.Fatalf("unexpected multi-word manufacturer row: %#v", quisk)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Label > got[i].Label {
			t.Fatalf("models not sorted by display label: %q > %q", got[i-1].Label, got[i].Label)
		}
	}
}

func TestParseHamlibCapabilitiesWSJTXInvariants(t *testing.T) {
	input := `Caps dump for model: 1020
Model name: FT-817
Mfg name: Yaesu
PTT type: RIG_PTT_RIG
Port type: RIG_PORT_SERIAL
Transceive: RIG_TRN_RIG
Can set_freq: Y
Can get_freq: Y
Can set_mode: Y
Can get_mode: Y
Can set_ptt: Y
Can get_ptt: Y
Can set_split_vfo: Y
Can get_split_vfo: Y
Can set_split_freq: Y
Can get_split_freq: Y
Mode list: USB LSB CW PKTUSB
`
	got := ParseHamlibCapabilities(input, 1020)
	if got.PortType != "serial" {
		t.Fatalf("port=%q want serial", got.PortType)
	}
	if !got.HasCATPTT {
		t.Fatal("CAT PTT should be enabled for RIG_PTT_RIG + set_ptt")
	}
	if !got.HasCATPTTMicData {
		t.Fatal("CAT audio-source capability should follow CAT PTT when Hamlib exposes no separate mic/data field")
	}
	if !got.Asynchronous {
		t.Fatal("RIG_TRN_RIG should mark CAT asynchronous")
	}
	if !got.HasSetSplitVFO || !got.HasGetSplitVFO || !got.HasSetSplitFreq || !got.HasGetSplitFreq {
		t.Fatal("split capabilities not parsed")
	}
	if len(got.SupportedModes) != 4 {
		t.Fatalf("supported modes=%v", got.SupportedModes)
	}
}

func TestParseHamlibCapabilitiesFailsClosed(t *testing.T) {
	got := ParseHamlibCapabilities("PTT type: RIG_PTT_NONE\nPort type: RIG_PORT_SERIAL\nCan set_ptt: N\nCan get_ptt: N\n", 99)
	if got.HasCATPTT || got.HasCATPTTMicData {
		t.Fatal("unsupported PTT capability must remain disabled")
	}
}
