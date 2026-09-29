package detector

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Tracker merges nearby spectral detections into stable signal candidates.
// JTTY decoding is deliberately not done here.
type Tracker struct {
	mu      sync.Mutex
	nextID  atomic.Uint64
	items   map[uint64]*Candidate
	freqTol float64
	ttl     time.Duration
}

func NewTracker(freqToleranceHz float64, ttl time.Duration) *Tracker {
	if freqToleranceHz <= 0 {
		freqToleranceHz = 20
	}
	if ttl <= 0 {
		ttl = 500 * time.Millisecond
	}
	return &Tracker{items: make(map[uint64]*Candidate), freqTol: freqToleranceHz, ttl: ttl}
}

// SetConfig applies live tracker tolerance and expiry values.
func (t *Tracker) SetConfig(freqToleranceHz float64, ttl time.Duration) {
	if freqToleranceHz < 1 {
		freqToleranceHz = 1
	}
	if ttl <= 0 {
		ttl = 500 * time.Millisecond
	}
	t.mu.Lock()
	t.freqTol = freqToleranceHz
	t.ttl = ttl
	t.mu.Unlock()
}

func (t *Tracker) Observe(frequencyHz, snr float64, at time.Time) Candidate {
	t.mu.Lock()
	defer t.mu.Unlock()

	var best *Candidate
	bestDist := math.MaxFloat64
	for _, c := range t.items {
		if at.Sub(c.LastSeen) > t.ttl {
			continue
		}
		d := math.Abs(c.FrequencyHz - frequencyHz)
		if d <= t.freqTol && d < bestDist {
			best = c
			bestDist = d
		}
	}

	if best == nil {
		id := t.nextID.Add(1)
		best = &Candidate{
			ID:          id,
			FrequencyHz: frequencyHz,
			SNR:         snr,
			FirstSeen:   at,
			LastSeen:    at,
			State:       StateSearching,
		}
		t.items[id] = best
	} else {
		best.FrequencyHz = 0.7*best.FrequencyHz + 0.3*frequencyHz
		best.SNR = 0.7*best.SNR + 0.3*snr
		best.LastSeen = at
		if best.State == StateSearching {
			best.State = StateSynced
		}
	}

	t.expireLocked(at)
	return *best
}

func (t *Tracker) expireLocked(now time.Time) {
	for id, c := range t.items {
		if now.Sub(c.LastSeen) > t.ttl {
			c.State = StateExpired
			delete(t.items, id)
		}
	}
}
