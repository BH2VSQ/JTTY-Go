package jtty

import (
	"strings"
	"sync/atomic"
)

// The limits/tolerances mirror the active-message state in WSJT-X
// lib/jtty/jtty_mdecode.f90. The assembler works above the PHY decoder: it
// turns one or more successfully decoded 59-symbol frames into a single live
// message update with a stable message ID.
const (
	MaxActiveMessages  = 30
	MaxRecentFrames    = MaxActiveMessages * MaxFrames
	MaxContinuationGap = 3
	MaxRetroSteps      = 3

	FrameHistoryTimeTolerance     = 0.05
	FrameHistoryFreqTolerance     = 3.0
	NearSimultaneousFreqTolerance = 12.0
	ContinuationTimeTolerance     = 0.1
)

type FrameDecode struct {
	FrequencyHz float64
	XDT         float64
	SyncUTC     float64
	SNRDB       float64
	Decoded     string
	TrailingSep bool
	LastFrame   bool
}

type MessageUpdate struct {
	MessageID   uint64
	FrequencyHz float64
	StartXDT    float64
	Text        string
	Complete    bool
}

type messageAssembly struct {
	ID          uint64
	FrequencyHz float64
	SyncXDT     float64
	StartXDT    float64
	Decoded     string
	TrailingSep bool
	LastFrame   bool
	FrameCount  int
}

type recentFrame struct {
	FrequencyHz float64
	SyncXDT     float64
}

// MessageAssembler mirrors the stateful part of WSJT-X jtty_mdecode. It is
// intentionally independent from GUI ordering: UI sequence numbers are added
// later by decode.Store.
var globalMessageID atomic.Uint64

// MessageAssembler owns only the currently active message state. Message IDs
// are allocated from a process-wide counter so restarting the receiver (or
// creating a new stream) can never reuse an old ID still present in decode.Store.
type MessageAssembler struct {
	active []messageAssembly
	recent []recentFrame
}

func NewMessageAssembler() *MessageAssembler {
	return &MessageAssembler{}
}

func (a *MessageAssembler) Reset() {
	a.active = a.active[:0]
	a.recent = a.recent[:0]
}

func (a *MessageAssembler) Add(frame FrameDecode, framePeriod float64) ([]MessageUpdate, bool) {
	if framePeriod <= 0 {
		framePeriod = float64(nFrameSymbols*nSS) / fs6k
	}
	if frame.Decoded == "" {
		return nil, false
	}

	for _, r := range a.recent {
		if sameRecentFrame(frame, r) {
			return nil, false
		}
	}

	best := -1
	bestGap := 1
	bestDF := 1e30
	for i := range a.active {
		match, gap, windowDupe := classifyActive(a.active[i], frame, framePeriod)
		if !match {
			continue
		}
		if windowDupe {
			return nil, false
		}
		df := abs(frame.FrequencyHz - a.active[i].FrequencyHz)
		if df < bestDF {
			best = i
			bestGap = gap
			bestDF = df
		}
	}

	a.remember(frame)
	if best >= 0 {
		msg := a.append(best, frame, bestGap)
		if msg == nil {
			return nil, false
		}
		return []MessageUpdate{{
			MessageID:   msg.ID,
			FrequencyHz: msg.FrequencyHz,
			StartXDT:    msg.StartXDT,
			Text:        displayMessageText(msg.Decoded),
			Complete:    frame.LastFrame,
		}}, true
	}

	msg := messageAssembly{
		ID:          globalMessageID.Add(1),
		FrequencyHz: frame.FrequencyHz,
		SyncXDT:     frame.XDT,
		StartXDT:    frame.XDT,
		Decoded:     frame.Decoded,
		TrailingSep: frame.TrailingSep,
		LastFrame:   frame.LastFrame,
		FrameCount:  1,
	}
	if strings.HasPrefix(msg.Decoded, "599 ") {
		msg.Decoded = "~" + msg.Decoded
	}
	update := MessageUpdate{MessageID: msg.ID, FrequencyHz: msg.FrequencyHz, StartXDT: msg.StartXDT, Text: displayMessageText(msg.Decoded), Complete: frame.LastFrame}
	if !frame.LastFrame && len(a.active) < MaxActiveMessages {
		a.active = append(a.active, msg)
	}
	return []MessageUpdate{update}, true
}

func (a *MessageAssembler) append(index int, frame FrameDecode, gap int) *messageAssembly {
	if index < 0 || index >= len(a.active) {
		return nil
	}
	m := &a.active[index]
	cur := strings.TrimRight(m.Decoded, " ")
	incoming := frame.Decoded
	if strings.HasPrefix(incoming, "~") {
		incoming = incoming[1:]
	}
	if gap > 1 {
		m.Decoded = cur + "~~~~~" + incoming
	} else if m.TrailingSep {
		m.Decoded = cur + " " + incoming
	} else {
		m.Decoded = cur + incoming
	}
	if len(m.Decoded) > 80 {
		m.Decoded = m.Decoded[:80]
	}
	m.TrailingSep = frame.TrailingSep
	m.FrequencyHz = frame.FrequencyHz
	m.SyncXDT = frame.XDT
	m.FrameCount++
	m.LastFrame = frame.LastFrame
	out := *m
	if frame.LastFrame {
		a.active[index] = a.active[len(a.active)-1]
		a.active = a.active[:len(a.active)-1]
	}
	return &out
}

func (a *MessageAssembler) remember(frame FrameDecode) {
	if len(a.recent) >= MaxRecentFrames {
		copy(a.recent, a.recent[1:])
		a.recent = a.recent[:MaxRecentFrames-1]
	}
	a.recent = append(a.recent, recentFrame{FrequencyHz: frame.FrequencyHz, SyncXDT: frame.XDT})
}

func (a *MessageAssembler) Prune(forwardXDT, framePeriod float64) {
	oldest := forwardXDT - float64(MaxRetroSteps)*framePeriod/4.0
	keep := a.recent[:0]
	for _, r := range a.recent {
		if r.SyncXDT >= oldest-FrameHistoryTimeTolerance {
			keep = append(keep, r)
		}
	}
	a.recent = keep
	for i := 0; i < len(a.active); {
		if oldest-a.active[i].SyncXDT > float64(MaxContinuationGap)*framePeriod+ContinuationTimeTolerance {
			a.active[i] = a.active[len(a.active)-1]
			a.active = a.active[:len(a.active)-1]
			continue
		}
		i++
	}
}

func sameRecentFrame(frame FrameDecode, r recentFrame) bool {
	return abs(frame.FrequencyHz-r.FrequencyHz) < NearSimultaneousFreqTolerance && abs(frame.XDT-r.SyncXDT) < FrameHistoryTimeTolerance
}

func classifyActive(existing messageAssembly, candidate FrameDecode, framePeriod float64) (match bool, gap int, windowDuplicate bool) {
	df := candidate.FrequencyHz - existing.FrequencyHz
	dt := candidate.XDT - existing.SyncXDT
	nfp := int(round(dt / framePeriod))
	if nfp >= 1 && nfp <= MaxContinuationGap && abs(dt-framePeriod*float64(nfp)) < ContinuationTimeTolerance {
		dfTol := 10.0 + 3.0*float64(nfp-1)
		if abs(df) < dfTol {
			return true, nfp, false
		}
	}
	qstep := framePeriod / 4
	nstep := int(round(dt / qstep))
	if abs(float64(nstep)) <= MaxRetroSteps && abs(df) < 10 && abs(dt-qstep*float64(nstep)) < 0.003 && !((nstep%4) == 0 && nstep != 0) {
		return true, 1, true
	}
	return false, 1, false
}

func displayMessageText(s string) string {
	for i := 0; i+5 <= len(s); i++ {
		if s[i:i+5] == "~~~~~" {
			s = s[:i] + " ... " + s[i+5:]
		}
	}
	s = strings.ReplaceAll(s, "~", " ")
	return strings.TrimSpace(s)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func round(v float64) float64 {
	if v >= 0 {
		return float64(int(v + 0.5))
	}
	return float64(int(v - 0.5))
}

func (a *MessageAssembler) ActiveCount() int { return len(a.active) }
