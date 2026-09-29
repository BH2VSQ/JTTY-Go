package receiver

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/audio"
	"github.com/BH2VSQ/jtty-go/internal/detector"
	"github.com/BH2VSQ/jtty-go/internal/waterfall"
)

type CandidateSink interface{ OnCandidates([]detector.Candidate) }
type WaterfallSink interface{ OnWaterfall(waterfall.Frame) }

type Pipeline struct {
	mu                 sync.Mutex
	scanner            *detector.Scanner
	tracker            *detector.Tracker
	sink               CandidateSink
	waterfall          WaterfallSink
	window             []float32
	hop                int
	startUTC           time.Time
	sampleRate         int
	windowStartSamples uint64
	jtty               *JTTYStream
	jobs               chan audio.PCMFrame
	stop               chan struct{}
	wg                 sync.WaitGroup
	scanCounter        uint32
	lastWaterfall      time.Time
	lastCandidateEmit  time.Time
	waterfallAnalyzer  *waterfall.Analyzer
	paused             atomic.Bool
}

func NewPipeline(scanner *detector.Scanner, tracker *detector.Tracker, sink CandidateSink, wf WaterfallSink, sampleRate, hop int) *Pipeline {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if hop <= 0 {
		hop = 1024
	}
	if scanner == nil {
		scanner = detector.NewScanner(detector.ScannerConfig{SampleRate: float64(sampleRate), MinFrequencyHz: 0, MaxFrequencyHz: float64(sampleRate / 2), FFTSize: 4096, ThresholdDb: 8})
	}
	p := &Pipeline{scanner: scanner, tracker: tracker, sink: sink, waterfall: wf, sampleRate: sampleRate, hop: hop, window: make([]float32, 0, scanner.FFTSize()*2), jobs: make(chan audio.PCMFrame, 12), stop: make(chan struct{})}
	if wf != nil {
		_, maxHz := scanner.FrequencyRange()
		// The waterfall is a display path, not the decode path. An 8192-point FFT
		// with a 2048-sample hop keeps roughly the same visual update cadence while
		// materially reducing FFT work and allocations on lower-end CPUs.
		p.waterfallAnalyzer = waterfall.NewAnalyzer(sampleRate, 0, maxHz, 8192, 4096)
	}
	p.wg.Add(1)
	go p.worker()
	return p
}

func (p *Pipeline) OnPCMFrame(frame audio.PCMFrame) { p.Process(frame) }

// Pause temporarily drops capture frames without tearing down the pipeline or
// its decoder workers. This is used for simplex TX so repeated transmissions do
// not repeatedly create/destroy the WASAPI/FFT/FEC stack.
func (p *Pipeline) Pause()       { p.paused.Store(true) }
func (p *Pipeline) Resume()      { p.paused.Store(false) }
func (p *Pipeline) Paused() bool { return p.paused.Load() }

func (p *Pipeline) AttachJTTY(stream *JTTYStream) {
	p.mu.Lock()
	p.jtty = stream
	p.mu.Unlock()
}

func (p *Pipeline) SetDecoderSettings(thresholdDb, frequencyToleranceHz, trackerToleranceHz float64, trackerTTL time.Duration) {
	p.mu.Lock()
	p.scanner.SetThresholdDB(thresholdDb)
	if p.tracker != nil {
		p.tracker.SetConfig(trackerToleranceHz, trackerTTL)
	}
	stream := p.jtty
	p.mu.Unlock()
	if stream != nil {
		stream.SetFrequencyTolerance(frequencyToleranceHz)
	}
}

func (p *Pipeline) SetFrequencyRange(minHz, maxHz float64) {
	p.mu.Lock()
	if p.scanner != nil {
		p.scanner.SetFrequencyRange(minHz, maxHz)
	}
	analyzer := p.waterfallAnalyzer
	p.mu.Unlock()
	if analyzer != nil {
		analyzer.SetFrequencyRange(minHz, maxHz)
	}
}

func (p *Pipeline) FrequencyRange() (float64, float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scanner.FrequencyRange()
}

// Process is the capture-thread boundary. It only copies and enqueues PCM so
// the audio callback never performs FFT, tracking, or FEC work.
func (p *Pipeline) Process(frame audio.PCMFrame) {
	if p.paused.Load() || len(frame.Samples) == 0 {
		return
	}
	if frame.SampleRate <= 0 {
		frame.SampleRate = p.sampleRate
	}
	if frame.SampleRate != p.sampleRate {
		frame.Samples = resampleLinear(frame.Samples, frame.SampleRate, p.sampleRate)
		frame.SampleRate = p.sampleRate
	}
	// The capture source owns each PCMFrame until OnPCMFrame returns, and the
	// current capture implementations provide a fresh sample slice per packet.
	// The pipeline only reads the slice, so a second full copy here would add GC
	// pressure and memory bandwidth without improving realtime isolation.
	select {
	case p.jobs <- frame:
	default:
		// Keep the processing queue close to real time: discard the oldest
		// pending frame before retrying, rather than letting UI/decode work
		// accumulate latency behind the live audio stream.
		select {
		case <-p.jobs:
		default:
		}
		select {
		case p.jobs <- frame:
		default:
		}
	}
}

func (p *Pipeline) Close() {
	select {
	case <-p.stop:
		return
	default:
		close(p.stop)
	}
	p.wg.Wait()
}

func (p *Pipeline) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.stop:
			return
		case frame := <-p.jobs:
			p.processFrame(frame)
		}
	}
}

func (p *Pipeline) processFrame(frame audio.PCMFrame) {
	p.mu.Lock()
	if p.startUTC.IsZero() {
		p.startUTC = frame.Timestamp.UTC()
		if p.startUTC.IsZero() {
			p.startUTC = time.Now().UTC()
		}
	}
	p.window = append(p.window, frame.Samples...)
	stream := p.jtty
	waterfallAnalyzer := p.waterfallAnalyzer
	p.mu.Unlock()

	// The live JTTY decoder must receive the actual capture stream. The old
	// pipeline only updated its frequency hints, so real input never reached
	// JTTYStream.Process.
	if stream != nil {
		stream.Process(frame.Samples, frame.Timestamp, frame.SampleRate)
	}
	if waterfallAnalyzer != nil && p.waterfall != nil {
		for _, wf := range waterfallAnalyzer.Process(frame.Samples, frame.Timestamp) {
			p.waterfall.OnWaterfall(wf)
		}
	}

	p.mu.Lock()
	for len(p.window) >= p.scanner.FFTSize() {
		window := p.window[:p.scanner.FFTSize()]
		at := p.startUTC.Add(time.Duration(p.windowStartSamples) * time.Second / time.Duration(p.sampleRate))
		p.scanCounter++
		shouldScan := p.scanCounter%2 == 0
		var stable []detector.Candidate
		var hints []float64
		if shouldScan {
			result := p.scanner.ScanFrame(window, at)
			if len(result.Power) > 0 {
				stable = make([]detector.Candidate, 0, len(result.Candidates))
				for _, c := range result.Candidates {
					if p.tracker != nil {
						stable = append(stable, p.tracker.Observe(c.FrequencyHz, c.SNR, at))
					} else {
						stable = append(stable, c)
					}
				}
				for _, c := range stable {
					if c.SNR < 0 {
						continue
					}
					hints = append(hints, c.FrequencyHz)
					if len(hints) == 4 {
						break
					}
				}
			}
		}
		shift := p.hop
		if shift > len(p.window) {
			shift = len(p.window)
		}
		copy(p.window, p.window[shift:])
		p.window = p.window[:len(p.window)-shift]
		p.windowStartSamples += uint64(shift)
		p.mu.Unlock()

		if stream != nil && shouldScan {
			stream.SetFrequencyHints(hints)
		}
		if len(stable) > 0 && p.sink != nil {
			p.mu.Lock()
			canEmit := p.lastCandidateEmit.IsZero() || at.Sub(p.lastCandidateEmit) >= 100*time.Millisecond
			if canEmit {
				p.lastCandidateEmit = at
			}
			p.mu.Unlock()
			if canEmit {
				p.sink.OnCandidates(stable)
			}
		}
		p.mu.Lock()
	}
	p.mu.Unlock()
}

func resampleLinear(in []float32, from, to int) []float32 {
	if len(in) == 0 || from <= 0 || to <= 0 || from == to {
		return append([]float32(nil), in...)
	}
	n := int(math.Round(float64(len(in)) * float64(to) / float64(from)))
	if n < 1 {
		n = 1
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		src := float64(i) * float64(from) / float64(to)
		j := int(src)
		frac := float32(src - float64(j))
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
		} else {
			out[i] = in[j]*(1-frac) + in[j+1]*frac
		}
	}
	return out
}
