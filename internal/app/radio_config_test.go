package app

import (
	"testing"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

func TestBuildRigctldArgsPTTMethods(t *testing.T) {
	cfg := model.RadioSettings{
		RigModelID: 1020, SerialPort: "COM3", Baud: 4800, DataBits: 8, StopBits: 1,
		Handshake: "None", PTTMethod: "Rigctl", ForceDTR: "none", ForceRTS: "none",
	}
	hasArg := func(args []string, value string) bool {
		for _, arg := range args {
			if arg == value {
				return true
			}
		}
		return false
	}
	hasPair := func(args []string, flag, value string) bool {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == flag && args[i+1] == value {
				return true
			}
		}
		return false
	}
	args := buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if !hasPair(args, "-m", "1020") || !hasPair(args, "-r", "COM3") || !hasPair(args, "-s", "4800") ||
		!hasArg(args, "data_bits=8,stop_bits=1,serial_handshake=None") || !hasArg(args, "ptt_type=RIG") {
		t.Fatalf("unexpected Rigctl args: %v", args)
	}

	cfg.PTTMethod = "VOX"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if !hasArg(args, "ptt_type=None") {
		t.Fatalf("VOX must disable Hamlib PTT: %v", args)
	}

	cfg.PTTMethod = "DTR"
	cfg.PTTSerialPort = "COM4"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if !hasArg(args, "ptt_type=DTR") || !hasArg(args, "ptt_pathname=COM4") {
		t.Fatalf("expected DTR PTT configuration: %v", args)
	}

	cfg.PTTMethod = "RTS"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if !hasArg(args, "ptt_type=RTS") || !hasArg(args, "ptt_pathname=COM4") {
		t.Fatalf("expected RTS PTT configuration: %v", args)
	}

	cfg.PTTSerialPort = "COM3"
	cfg.ForceDTR = "on"
	cfg.PTTMethod = "DTR"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if hasArg(args, "dtr_state=ON") {
		t.Fatalf("forced DTR state must not conflict with DTR PTT: %v", args)
	}
	cfg.ForceDTR = "none"
	cfg.ForceRTS = "on"
	cfg.PTTMethod = "RTS"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	if hasArg(args, "rts_state=ON") {
		t.Fatalf("forced RTS state must not conflict with RTS PTT: %v", args)
	}
}
