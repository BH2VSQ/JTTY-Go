package receiver

import (
	"sync"
	"testing"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/jtty"
	"github.com/BH2VSQ/jtty-go/internal/model"
)

type jttyDecodeCollector struct {
	mu   sync.Mutex
	list []model.DecodeMessage
	ch   chan model.DecodeMessage
}

func (s *jttyDecodeCollector) OnDecodeMessage(m model.DecodeMessage) {
	s.mu.Lock()
	s.list = append(s.list, m)
	s.mu.Unlock()
	select {
	case s.ch <- m:
	default:
	}
}

func TestJTTYStreamRealtimeDecode(t *testing.T) {
	frames, _, err := jtty.PackMessage("CQ K1ABC CQ", jtty.ExchangeUnknown)
	if err != nil || len(frames) != 1 {
		t.Fatalf("pack: frames=%d err=%v", len(frames), err)
	}
	var payload [jtty.PayloadBits]int
	for i := range payload {
		if frames[0].Bits&(uint64(1)<<uint(jtty.PayloadBits-1-i)) != 0 {
			payload[i] = 1
		}
	}
	tones := jtty.EncodeFrame(payload)
	wave12 := jtty.GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
	// rjtty_core waits for a 1.25-frame window. Append 0.25 frame of silence.
	full12 := make([]float32, JTTYChunkSamples12k)
	copy(full12, wave12)
	for i := range full12 {
		full12[i] *= 12000
	}
	// Simulate a 48 kHz device by holding each 12 kHz sample for four samples.
	full48 := make([]float32, len(full12)*4)
	for i, v := range full12 {
		base := i * 4
		full48[base], full48[base+1], full48[base+2], full48[base+3] = v, v, v, v
	}

	collector := &jttyDecodeCollector{ch: make(chan model.DecodeMessage, 8)}
	stream := NewJTTYStream(jtty.NewDecoder(), collector, 1500, 4, 2)
	defer stream.Close()

	start := time.Unix(1700000000, 123000000).UTC()
	// Deliberately use odd packet sizes to verify the decimator carry path.
	for pos := 0; pos < len(full48); {
		n := 997
		if remain := len(full48) - pos; remain < n {
			n = remain
		}
		stream.Process(full48[pos:pos+n], start.Add(time.Duration(pos)*time.Second/48000), 48000)
		pos += n
	}

	select {
	case got := <-collector.ch:
		if got.Message != "CQ K1ABC CQ" {
			t.Fatalf("message=%q", got.Message)
		}
		if got.FrequencyHz != 1500 {
			t.Fatalf("frequency=%d", got.FrequencyHz)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for realtime JTTY decode")
	}
}

func TestJTTYStreamDecodesQuarterFrameOffset(t *testing.T) {
	frames, _, err := jtty.PackMessage("CQ K1ABC CQ", jtty.ExchangeUnknown)
	if err != nil || len(frames) != 1 {
		t.Fatalf("pack: frames=%d err=%v", len(frames), err)
	}
	var payload [jtty.PayloadBits]int
	for i := range payload {
		if frames[0].Bits&(uint64(1)<<uint(jtty.PayloadBits-1-i)) != 0 {
			payload[i] = 1
		}
	}
	tones := jtty.EncodeFrame(payload)
	wave12 := jtty.GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
	lead := int(0.100 * JTTYAudioRate)
	full12 := make([]float32, JTTYChunkSamples12k)
	copy(full12[lead:], wave12)
	for i := range full12 {
		full12[i] *= 12000
	}
	full48 := make([]float32, len(full12)*4)
	for i, v := range full12 {
		base := i * 4
		full48[base], full48[base+1], full48[base+2], full48[base+3] = v, v, v, v
	}
	collector := &jttyDecodeCollector{ch: make(chan model.DecodeMessage, 8)}
	stream := NewJTTYStream(jtty.NewDecoder(), collector, 1500, 4, 2)
	defer stream.Close()
	start := time.Unix(1700000000, 0).UTC()
	for pos := 0; pos < len(full48); {
		n := 997
		if remain := len(full48) - pos; remain < n {
			n = remain
		}
		stream.Process(full48[pos:pos+n], start.Add(time.Duration(pos)*time.Second/48000), 48000)
		pos += n
	}
	select {
	case got := <-collector.ch:
		if got.Message != "CQ K1ABC CQ" {
			t.Fatalf("message=%q", got.Message)
		}
		if got.DT < 0.08 || got.DT > 0.12 {
			t.Fatalf("unexpected DT=%.3f", got.DT)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for offset realtime JTTY decode")
	}
}
