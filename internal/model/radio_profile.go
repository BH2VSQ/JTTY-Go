package model

import "strings"

const RadioConfigVersion = 2

// Profile snapshots only connection and operating parameters, never nested profiles.
func (r RadioSettings) Profile() RadioProfile {
	return RadioProfile{RigName: r.RigName, RigModelID: r.RigModelID, RigctldPath: r.RigctldPath, Host: r.Host, Port: r.Port, SerialPort: r.SerialPort, Baud: r.Baud, DataBits: r.DataBits, StopBits: r.StopBits, Handshake: r.Handshake, PTTMethod: r.PTTMethod, PTTSerialPort: r.PTTSerialPort, SplitMode: r.SplitMode, TXAudioSource: r.TXAudioSource, ForceDTR: r.ForceDTR, ForceRTS: r.ForceRTS, PollIntervalSec: r.PollIntervalSec, Mode: r.Mode, PassbandHz: r.PassbandHz, ReadPowerSWR: r.ReadPowerSWR, HaltOnSWR: r.HaltOnSWR}
}

func (r *RadioSettings) ApplyProfile(p RadioProfile) {
	r.RigName = p.RigName
	r.RigModelID = p.RigModelID
	r.RigctldPath = p.RigctldPath
	r.Host = p.Host
	r.Port = p.Port
	r.SerialPort = p.SerialPort
	r.Baud = p.Baud
	r.DataBits = p.DataBits
	r.StopBits = p.StopBits
	r.Handshake = p.Handshake
	r.PTTMethod = p.PTTMethod
	r.PTTSerialPort = p.PTTSerialPort
	r.SplitMode = p.SplitMode
	r.TXAudioSource = p.TXAudioSource
	r.ForceDTR = p.ForceDTR
	r.ForceRTS = p.ForceRTS
	r.PollIntervalSec = p.PollIntervalSec
	r.Mode = p.Mode
	r.PassbandHz = p.PassbandHz
	r.ReadPowerSWR = p.ReadPowerSWR
	r.HaltOnSWR = p.HaltOnSWR
}

// NormalizeRadio preserves explicit backend selection. Legacy NET settings are
// never inferred to be local just because a model ID is still present.
func NormalizeRadio(r RadioSettings) RadioSettings {
	backend := strings.ToLower(strings.TrimSpace(r.Backend))
	legacyNone := backend == "hamlib-rigctld" && r.RigModelID <= 0 && !r.Enabled
	switch backend {
	case "hamlib-local":
		r.Backend = "hamlib-local"
	case "hamlib-rigctld":
		if r.RigModelID > 0 {
			r.Backend = "hamlib-local"
		} else {
			r.Backend = "none"
		}
	case "rigctld-net", "hamlib-net-rigctl":
		r.Backend = "rigctld-net"
	default:
		r.Backend = "none"
		if r.Enabled {
			r.Backend = "rigctld-net"
		}
	}
	r.Enabled = r.Backend != "none"
	if legacyNone {
		r.PTTMethod = "VOX"
	}
	switch strings.ToUpper(strings.TrimSpace(r.PTTMethod)) {
	case "CAT", "RIGCTL":
		r.PTTMethod = "Rigctl"
	case "DTR":
		r.PTTMethod = "DTR"
	case "RTS":
		r.PTTMethod = "RTS"
	default:
		r.PTTMethod = "VOX"
	}
	if r.Host == "" {
		r.Host = "127.0.0.1"
	}
	if r.Port < 1 || r.Port > 65535 {
		r.Port = 4532
	}
	if r.PollIntervalSec < 1 {
		r.PollIntervalSec = 1
	}
	if r.PollIntervalSec > 30 {
		r.PollIntervalSec = 30
	}
	if r.SplitMode == "Fake It" {
		r.SplitMode = "None"
	}
	if r.Local == (RadioProfile{}) {
		r.Local = RadioProfile{Mode: "PKTUSB", PassbandHz: 3000, Baud: 9600, DataBits: 8, StopBits: 1, Handshake: "None", PTTMethod: "VOX", SplitMode: "None", PollIntervalSec: 1, ForceDTR: "none", ForceRTS: "none", TXAudioSource: "Front"}
	}
	if r.NET == (RadioProfile{}) {
		r.NET = r.Profile()
		r.NET.RigModelID = 0
		r.NET.RigName = "NET rigctl"
	}
	if r.Backend == "hamlib-local" {
		r.Local = r.Profile()
	}
	if r.Backend == "rigctld-net" {
		r.NET = r.Profile()
	}
	r.ConfigVersion = RadioConfigVersion
	return r
}
