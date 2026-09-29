package detector

import "time"

type CandidateState uint8

const (
	StateSearching CandidateState = iota
	StateSynced
	StateCollecting
	StateDecoding
	StateConfirmed
	StateExpired
)

type Candidate struct {
	ID          uint64
	FrequencyHz float64
	SNR         float64
	FirstSeen   time.Time
	LastSeen    time.Time
	SyncScore   float64
	Confidence  float64
	State       CandidateState
}
