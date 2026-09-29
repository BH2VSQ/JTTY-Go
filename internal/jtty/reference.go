package jtty

// WSJT-X JTTY reference metadata. The current project is intended to use the
// JTTY implementation in WSJT-X 3.2.0-rc1 as the interoperability reference,
// while the Go implementation remains modular and independently testable.
type Reference struct {
	Project     string
	Release     string
	Commit      string
	License     string
	Mode        string
	Baud        float64
	Tones       int
	BandwidthHz float64
	FEC         string
}

var WSJTXReference = Reference{
	Project:     "WSJT-X",
	Release:     "3.2.0-rc1",
	Commit:      "567ad29",
	License:     "GPL-3.0",
	Mode:        "JTTY",
	Baud:        31.25,
	Tones:       4,
	BandwidthHz: 125,
	FEC:         "K=10, r=1/2, (92,46)",
}
