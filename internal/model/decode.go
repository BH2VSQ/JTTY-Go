package model

import "time"

type DecodeMessage struct {
	Sequence    uint64    `json:"sequence"`
	SignalUTC   time.Time `json:"signalUtc"`
	ReceivedUTC time.Time `json:"receivedUtc"`

	SNR         int     `json:"snr"`
	DT          float64 `json:"dt"`
	FrequencyHz int     `json:"frequencyHz"`

	Message    string     `json:"message"`
	Callsigns  []Callsign `json:"callsigns,omitempty"`
	Confidence float64    `json:"confidence"`

	// JTTYMessageID keeps the WSJT-X-style live message identity across frame
	// updates. The PHY details below are internal to the Go assembler and are
	// deliberately not part of the JSON/UI contract.
	JTTYMessageID uint64 `json:"jttyMessageId,omitempty"`
	JTTYLastFrame bool   `json:"-"`
	JTTYTrailing  bool   `json:"-"`
}

type Callsign struct {
	Value      string  `json:"value"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}
