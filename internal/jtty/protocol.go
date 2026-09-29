package jtty

// Protocol exposes the JTTY operating characteristics used by the Go PHY.
// Detailed source packing, TBCC coding, sync correlation and waveform
// generation are implemented in the jtty package rather than represented as
// configuration values.
type Protocol struct {
	Baud              float64
	Tones             int
	BandwidthHz       float64
	TxDurationSeconds float64
}

var CurrentProtocol = Protocol{
	Baud:              31.25,
	Tones:             4,
	BandwidthHz:       125,
	TxDurationSeconds: 1.888,
}
