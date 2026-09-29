package receiver

import (
	"testing"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/audio"
	"github.com/BH2VSQ/jtty-go/internal/detector"
)

type candidateSink struct{ n int }

func (s *candidateSink) OnCandidates(c []detector.Candidate) { s.n += len(c) }

func TestPipelineOverlappingWindows(t *testing.T) {
	scanner := detector.NewScanner(detector.ScannerConfig{SampleRate: 48000, MinFrequencyHz: 300, MaxFrequencyHz: 2700, FFTSize: 1024, ThresholdDb: 1})
	tracker := detector.NewTracker(20, 500*time.Millisecond)
	cs := &candidateSink{}
	p := NewPipeline(scanner, tracker, cs, nil, 48000, 256)
	defer p.Close()
	p.Process(audio.PCMFrame{Timestamp: time.Unix(0, 0).UTC(), Samples: make([]float32, 3000), SampleRate: 48000, Channels: 1})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		advanced := p.windowStartSamples > 0
		p.mu.Unlock()
		if advanced {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatal("pipeline did not advance window")
}

func TestPipelinePauseDropsFramesUntilResume(t *testing.T) {
	scanner := detector.NewScanner(detector.ScannerConfig{SampleRate: 48000, MinFrequencyHz: 300, MaxFrequencyHz: 2700, FFTSize: 1024, ThresholdDb: 1})
	tracker := detector.NewTracker(20, 500*time.Millisecond)
	cs := &candidateSink{}
	p := NewPipeline(scanner, tracker, cs, nil, 48000, 256)
	defer p.Close()
	p.Pause()
	p.Process(audio.PCMFrame{Timestamp: time.Unix(0, 0).UTC(), Samples: make([]float32, 3000), SampleRate: 48000, Channels: 1})
	time.Sleep(10 * time.Millisecond)
	p.mu.Lock()
	pausedLen := len(p.window)
	p.mu.Unlock()
	if pausedLen != 0 {
		t.Fatalf("paused pipeline processed %d samples", pausedLen)
	}
	p.Resume()
	p.Process(audio.PCMFrame{Timestamp: time.Unix(0, 0).UTC(), Samples: make([]float32, 3000), SampleRate: 48000, Channels: 1})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		advanced := p.windowStartSamples > 0
		p.mu.Unlock()
		if advanced {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatal("resumed pipeline did not process frame")
}
