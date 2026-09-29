package app

import (
	"fmt"
	"testing"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

func TestBuildRigctldArgsWSJTXSemantics(t *testing.T) {
	cfg := model.RadioSettings{
		RigModelID:    1020,
		SerialPort:    "COM3",
		Baud:          4800,
		DataBits:      8,
		StopBits:      1,
		Handshake:     "None",
		PTTMethod:     "VOX",
		ForceDTR:      "none",
		ForceRTS:      "none",
		PTTSerialPort: "",
	}
	args := buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	joined := fmt.Sprint(args)
	hasPair := func(flag, value string) bool {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == flag && args[i+1] == value {
				return true
			}
		}
		return false
	}
	hasArg := func(value string) bool {
		for _, arg := range args {
			if arg == value {
				return true
			}
		}
		return false
	}
	if !hasPair("-m", "1020") || !hasPair("-r", "COM3") || !hasPair("-s", "4800") ||
		!hasArg("data_bits=8,stop_bits=1,serial_handshake=None") || !hasArg("ptt_type=None") {
		t.Fatalf("unexpected rigctld args: %v", joined)
	}

	cfg.PTTMethod = "DTR"
	cfg.PTTSerialPort = "COM4"
	args = buildRigctldArgs(cfg, "127.0.0.1", 4532, "COM3")
	hasType, hasPath := false, false
	for _, arg := range args {
		hasType = hasType || arg == "ptt_type=DTR"
		hasPath = hasPath || arg == "ptt_pathname=COM4"
	}
	if !hasType || !hasPath {
		t.Fatalf("expected DTR PTT configuration in args: %v", args)
	}
}
