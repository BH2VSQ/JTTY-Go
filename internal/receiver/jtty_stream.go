package receiver

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/decoder"
	"github.com/BH2VSQ/jtty-go/internal/jtty"
	"github.com/BH2VSQ/jtty-go/internal/model"
)

const (
	JTTYAudioRate       = 12000
	JTTYFrameSamples12k = 59 * 384
	JTTYStepSamples12k  = JTTYFrameSamples12k / 4
	// Keep one extra quarter-frame in every live search window. The decoder
	// searches synchronization timing across the first quarter-frame, so an
	// exact one-frame window would only decode signals that happened to start
	// at the window boundary.
	JTTYChunkSamples12k = JTTYFrameSamples12k + JTTYStepSamples12k
	JTTYRetroSamples12k = JTTYStepSamples12k * jtty.MaxRetroSteps
)

type DecodeSink interface{ OnDecodeMessage(model.DecodeMessage) }

type jttyWindowJob struct {
	sequence    uint64
	startSample int64
	startUTC    time.Time
	samples     []float32
	centerHz    float64
	centers     []float64
	retro       bool
	interferers []jtty.KnownInterferer
	retroKey    string
}

type jttyWindowResult struct {
	sequence    uint64
	startSample int64
	startUTC    time.Time
	frames      []jtty.DecodedFrame
	retro       bool
	retroKey    string
}

// JTTYStream turns 48 kHz capture into the 12 kHz input expected by the
// WSJT-X JTTY decoder, keeps enough history for three retro quarter-frame
// sweeps, and runs decoder windows asynchronously so the audio callback never
// waits for DSP/FEC work.
type JTTYStream struct {
	decoder       *jtty.Decoder
	assembler     *jtty.MessageAssembler
	sink          DecodeSink
	centerHz      float64
	centers       []float64
	maxCandidates int
	workers       int

	mu         sync.Mutex
	buf        []float32
	baseSample int64
	nextStart  int64
	startUTC   time.Time
	seq        uint64
	decimCarry []float32
	decimWork  []float32
	history    map[uint64]jttyWindowJob
	historySeq []uint64
	retroSeen  map[string]struct{}

	jobs            chan jttyWindowJob
	retroJobs       chan jttyWindowJob
	wideJobs        chan jttyWindowJob
	results         chan jttyWindowResult
	stop            chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	latestQueuedSeq atomic.Uint64
	activeForward   atomic.Int32
	activeRetro     atomic.Int32
}

func NewJTTYStream(dec *jtty.Decoder, sink DecodeSink, centerHz float64, maxCandidates, workers int) *JTTYStream {
	if dec == nil {
		dec = jtty.NewDecoder()
	}
	if centerHz <= 0 {
		centerHz = 1500
	}
	if maxCandidates < 1 {
		maxCandidates = 4
	}
	if maxCandidates > 8 {
		maxCandidates = 8
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &JTTYStream{
		decoder: dec, assembler: jtty.NewMessageAssembler(), sink: sink,
		centerHz: centerHz, maxCandidates: maxCandidates, workers: workers,
		buf:       make([]float32, 0, JTTYChunkSamples12k+JTTYRetroSamples12k+JTTYStepSamples12k),
		jobs:      make(chan jttyWindowJob, workers*2),
		retroJobs: make(chan jttyWindowJob, workers*2),
		wideJobs:  make(chan jttyWindowJob, 1),
		results:   make(chan jttyWindowResult, workers*2),
		stop:      make(chan struct{}), ctx: ctx, cancel: cancel,
		history: make(map[uint64]jttyWindowJob), retroSeen: make(map[string]struct{}),
	}
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	// Keep full-band discovery off the realtime worker pool. The v5.29 PHY decoder
	// performs an expensive wide search, so running it on every 470 ms live window
	// can starve the receiver on a normal desktop.
	s.wg.Add(1)
	go s.wideDiscoveryWorker()
	s.wg.Add(1)
	go s.collector()
	return s
}

func (s *JTTYStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
	s.wg.Wait()
}

func (s *JTTYStream) SetCenterFrequency(hz float64) {
	s.mu.Lock()
	if hz > 0 {
		s.centerHz = hz
	}
	s.mu.Unlock()
}

func (s *JTTYStream) SetFrequencyTolerance(hz float64) {
	if s.decoder != nil {
		s.decoder.SetFrequencyTolerance(hz)
	}
}

func (s *JTTYStream) SetMinSNR(snr float64) {
	if s.decoder != nil {
		s.decoder.SetMinSNR(snr)
	}
}

func (s *JTTYStream) SetFrequencyHints(hints []float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(hints) > 4 {
		hints = hints[:4]
	}
	s.centers = append(s.centers[:0], hints...)
}

func (s *JTTYStream) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = s.buf[:0]
	s.baseSample = 0
	s.nextStart = 0
	s.startUTC = time.Time{}
	s.seq = 0
	s.assembler.Reset()
	clear(s.history)
	s.historySeq = s.historySeq[:0]
	clear(s.retroSeen)
}

func (s *JTTYStream) Process(frame []float32, timestamp time.Time, sampleRate int) {
	if len(frame) == 0 {
		return
	}
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if sampleRate != 48000 {
		frame = resampleLinear12(frame, sampleRate, 48000)
	}
	s.mu.Lock()
	before := len(s.buf)
	s.appendDecimated4Locked(frame)
	if len(s.buf) == before {
		s.mu.Unlock()
		return
	}
	if s.startUTC.IsZero() {
		s.startUTC = timestamp.UTC()
	}
	for s.nextStart+JTTYChunkSamples12k <= s.baseSample+int64(len(s.buf)) {
		local := s.nextStart - s.baseSample
		if local < 0 {
			s.nextStart = s.baseSample
			local = 0
		}
		start := int(local)
		end := start + JTTYChunkSamples12k
		if end > len(s.buf) {
			break
		}
		window := append([]float32(nil), s.buf[start:end]...)
		windowUTC := s.startUTC.Add(time.Duration(s.nextStart) * time.Second / JTTYAudioRate)
		seq := s.seq + 1
		// Keep a low-rate full-band fallback even when detector hints exist. The
		// detector is intentionally advisory; periodic wide sweeps preserve
		// discovery coverage while reducing the normal hinted path to about one
		// sweep per eight windows. When no hints exist, retain a one-in-four
		// fallback so a quiet spectrum can still be discovered.
		wideSweep := (len(s.centers) == 0 && seq%4 == 0) || (len(s.centers) > 0 && seq%8 == 0)
		job := jttyWindowJob{sequence: seq, startSample: s.nextStart, samples: window, startUTC: windowUTC, centerHz: s.centerHz, centers: append([]float64(nil), s.centers...)}
		queued := false
		select {
		case s.jobs <- job:
			queued = true
		default:
			// Realtime rule: if the forward queue is full, discard one stale
			// queued window and keep the newest window instead. A stale queued
			// window is less useful than the latest audio and was the primary
			// source of visible decode lag under heavy CPU load.
			select {
			case dropped := <-s.jobs:
				delete(s.history, dropped.sequence)
				for i, histSeq := range s.historySeq {
					if histSeq == dropped.sequence {
						s.historySeq = append(s.historySeq[:i], s.historySeq[i+1:]...)
						break
					}
				}
			default:
			}
			select {
			case s.jobs <- job:
				queued = true
			default:
			}
		}
		if queued {
			s.seq = seq
			s.latestQueuedSeq.Store(seq)
			s.history[seq] = job
			if wideSweep {
				wideJob := job
				wideJob.retro = true
				wideJob.retroKey = fmt.Sprintf("wide:%d", job.sequence)
				select {
				case s.wideJobs <- wideJob:
				default:
				}
			}

			s.historySeq = append(s.historySeq, seq)
			for len(s.historySeq) > jtty.MaxRetroSteps+4 {
				old := s.historySeq[0]
				delete(s.history, old)
				s.historySeq = s.historySeq[1:]
			}
			if len(s.retroSeen) > 1024 {
				// Retro keys only protect short-lived history pairs. Once the map grows
				// beyond this bound, all keys refer to windows older than the retained
				// history horizon, so dropping the set is safe and keeps memory bounded.
				clear(s.retroSeen)
			}
		}
		s.nextStart += JTTYStepSamples12k
		dropBefore := s.nextStart - JTTYRetroSamples12k
		if dropBefore > s.baseSample {
			drop := int(dropBefore - s.baseSample)
			if drop > len(s.buf) {
				drop = len(s.buf)
			}
			s.buf = append(s.buf[:0], s.buf[drop:]...)
			s.baseSample = dropBefore
		}
	}
	s.mu.Unlock()
}

// DecodeNow queues the newest complete JTTY window immediately. It is used
// by the explicit Decode button without changing the live decode scheduler.
func (s *JTTYStream) DecodeNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	available := s.baseSample + int64(len(s.buf)) - s.nextStart
	if available < JTTYFrameSamples12k {
		return false
	}
	startAbs := s.baseSample + int64(len(s.buf)) - JTTYFrameSamples12k
	local := int(startAbs - s.baseSample)
	if local < 0 {
		local = 0
	}
	end := local + JTTYFrameSamples12k
	if end > len(s.buf) {
		return false
	}
	window := append([]float32(nil), s.buf[local:end]...)
	windowUTC := s.startUTC.Add(time.Duration(startAbs) * time.Second / JTTYAudioRate)
	seq := s.seq + 1
	job := jttyWindowJob{sequence: seq, startSample: startAbs, samples: window, startUTC: windowUTC, centerHz: s.centerHz, centers: append([]float64(nil), s.centers...)}
	select {
	case s.jobs <- job:
		s.seq = seq
		s.latestQueuedSeq.Store(seq)
		s.history[seq] = job
		s.historySeq = append(s.historySeq, seq)
		return true
	default:
		// Explicit Decode must not jump ahead of the already-running realtime
		// queue, so it reports busy instead of adding more backlog.
		return false
	}
}

func (s *JTTYStream) appendDecimated4Locked(in []float32) {
	if len(in) == 0 {
		return
	}
	// Consume the at-most-three carry samples first. Once a complete group is
	// available, append one output sample directly to the 12 kHz history. This
	// avoids copying every capture packet into a second scratch buffer.
	i := 0
	if len(s.decimCarry) > 0 {
		need := 4 - len(s.decimCarry)
		if need > len(in) {
			s.decimCarry = append(s.decimCarry, in...)
			return
		}
		var sum float32
		for _, v := range s.decimCarry {
			sum += v
		}
		for j := 0; j < need; j++ {
			sum += in[i+j]
		}
		s.buf = append(s.buf, sum*0.25)
		i += need
		s.decimCarry = s.decimCarry[:0]
	}
	full := (len(in) - i) / 4
	for n := 0; n < full; n++ {
		j := i + n*4
		s.buf = append(s.buf, (in[j]+in[j+1]+in[j+2]+in[j+3])*0.25)
	}
	i += full * 4
	if i < len(in) {
		s.decimCarry = append(s.decimCarry[:0], in[i:]...)
	}
}

func (s *JTTYStream) worker() {
	defer s.wg.Done()
	for {
		var job jttyWindowJob
		// Forward windows have strict realtime priority. Retro sweeps are
		// optional refinement work and must never build latency behind live audio.
		select {
		case <-s.stop:
			return
		case job = <-s.jobs:
			// Prefer a pending forward window when one is immediately available.
		default:
			select {
			case <-s.stop:
				return
			case job = <-s.jobs:
			case job = <-s.retroJobs:
			}
		}
		if job.retro {
			// Optional retro work only runs while the live queue is completely idle.
			// This keeps recovery decoding from stealing CPU after a new audio window arrives.
			if s.activeRetro.Add(1) > 1 {
				s.activeRetro.Add(-1)
				continue
			}
			if len(s.jobs) > 0 || s.activeForward.Load() > 0 {
				s.activeRetro.Add(-1)
				continue
			}
		} else {
			s.activeForward.Add(1)
		}
		// The spectral scanner publishes up to four hints for the UI, but using
		// those hints as four independent JTTY decodes caused a large CPU queue:
		// Decode the configured QSO frequency first. Spectrum-tracker hints are
		// supplementary anchors, not replacements for the operator-selected
		// center frequency. This keeps the v5.29 PHY decoder responsive to the
		// actual JTTY receive channel without paying the full-band cost every window.
		decodeJob := decoder.Job{CandidateID: job.sequence, Samples: job.samples, FrequencyHz: job.centerHz, Timestamp: job.startUTC}
		var frames []jtty.DecodedFrame
		if job.centerHz > 0 {
			narrow, err := s.decoder.DecodeManyDetailed(s.ctx, decodeJob, minInt(2, s.maxCandidates), true, job.interferers)
			if err == nil && len(narrow) > 0 {
				frames = narrow
			}
		}
		if len(frames) == 0 {
			anchors := make([]float64, 0, 4)
			for _, center := range job.centers {
				if center <= 0 {
					continue
				}
				duplicate := false
				for _, prev := range anchors {
					if math.Abs(center-prev) < 25 {
						duplicate = true
						break
					}
				}
				if math.Abs(center-job.centerHz) < 25 {
					duplicate = true
				}
				if duplicate {
					continue
				}
				anchors = append(anchors, center)
				decodeJob.FrequencyHz = center
				narrow, err := s.decoder.DecodeManyDetailed(s.ctx, decodeJob, minInt(2, s.maxCandidates), true, job.interferers)
				if err == nil && len(narrow) > 0 {
					frames = narrow
					break
				}
				if len(anchors) >= 3 {
					break
				}
			}
		}
		if job.retro {
			s.activeRetro.Add(-1)
		} else {
			s.activeForward.Add(-1)
		}
		result := jttyWindowResult{sequence: job.sequence, startSample: job.startSample, startUTC: job.startUTC, frames: frames, retro: job.retro, retroKey: job.retroKey}
		select {
		case s.results <- result:
		case <-s.stop:
			return
		}
	}
}

func (s *JTTYStream) wideDiscoveryWorker() {
	defer s.wg.Done()
	for {
		var job jttyWindowJob
		select {
		case <-s.stop:
			return
		case job = <-s.wideJobs:
		}
		if len(s.jobs) > 0 || s.activeForward.Load() > 0 {
			continue
		}
		frames, _ := s.decoder.DecodeWidebandManyDetailed(s.ctx, decoder.Job{CandidateID: job.sequence, Samples: job.samples, FrequencyHz: job.centerHz, Timestamp: job.startUTC}, s.maxCandidates, true, job.interferers)
		result := jttyWindowResult{sequence: job.sequence, startSample: job.startSample, startUTC: job.startUTC, frames: frames, retro: true, retroKey: job.retroKey}
		select {
		case s.results <- result:
		case <-s.stop:
			return
		}
	}
}

func (s *JTTYStream) collector() {
	defer s.wg.Done()
	var lastForward uint64
	for {
		select {
		case <-s.stop:
			return
		case r := <-s.results:
			if r.retro {
				s.consume(r)
				continue
			}
			// Forward decode is deliberately freshness-first. Worker completion
			// order can differ under load, and waiting for every missing sequence
			// would make a fast current result sit behind one slow old window. Any
			// result older than the newest consumed forward window is stale and is
			// discarded.
			if r.sequence <= lastForward {
				continue
			}
			lastForward = r.sequence
			s.consume(r)
		}
	}
}

func (s *JTTYStream) consume(result jttyWindowResult) {
	framePeriod := float64(JTTYFrameSamples12k) / JTTYAudioRate
	s.mu.Lock()
	defer s.mu.Unlock()
	forward := float64(result.startSample) / JTTYAudioRate
	if !result.retro {
		s.assembler.Prune(forward, framePeriod)
	}
	for _, hit := range result.frames {
		message := hit.Message
		frame := jtty.FrameDecode{
			FrequencyHz: hit.FrequencyHz,
			XDT:         forward + hit.XDT,
			SNRDB:       float64(message.SNR),
			Decoded:     message.Message,
			LastFrame:   message.JTTYLastFrame,
			TrailingSep: message.JTTYTrailing,
		}
		updates, accepted := s.assembler.Add(frame, framePeriod)
		if !accepted {
			continue
		}
		for _, update := range updates {
			item := model.DecodeMessage{
				SignalUTC:     result.startUTC.Add(time.Duration((update.StartXDT - forward) * float64(time.Second))),
				ReceivedUTC:   time.Now().UTC(),
				SNR:           message.SNR,
				DT:            message.DT,
				FrequencyHz:   int(update.FrequencyHz + 0.5),
				Message:       update.Text,
				Callsigns:     jtty.ExtractCallsigns(update.Text),
				Confidence:    message.Confidence,
				JTTYMessageID: update.MessageID,
				JTTYLastFrame: update.Complete,
			}
			if s.sink != nil {
				s.sink.OnDecodeMessage(item)
			}
		}
	}
	if !result.retro && len(result.frames) > 0 {
		s.scheduleRetroLocked(result, forward)
	}
}

func (s *JTTYStream) scheduleRetroLocked(result jttyWindowResult, currentForward float64) {
	if result.sequence == 0 {
		return
	}
	interferers := make([]jtty.KnownInterferer, 0, minInt(len(result.frames), 8))
	for _, hit := range result.frames {
		interferers = append(interferers, jtty.KnownInterferer{
			FrequencyHz: hit.FrequencyHz,
			XDT:         currentForward + hit.XDT,
			Tones:       hit.Tones,
		})
		if len(interferers) == 8 {
			break
		}
	}
	// Only schedule one nearest retro revisit. It is enough to recover an
	// overlapping frame while preventing a successful live decode from spawning
	// several expensive background PHY jobs. Older history remains available for
	// a later explicit/idle revisit.
	if len(s.retroJobs) > 0 || len(s.jobs) > 0 || s.activeForward.Load() > 0 || s.activeRetro.Load() > 0 {
		return
	}
	for k := 1; k <= 1; k++ {
		if result.sequence <= uint64(k) {
			break
		}
		targetSeq := result.sequence - uint64(k)
		target, ok := s.history[targetSeq]
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d:%d", result.sequence, targetSeq)
		if _, seen := s.retroSeen[key]; seen {
			continue
		}
		localInterferers := make([]jtty.KnownInterferer, 0, len(interferers))
		targetForward := float64(target.startSample) / JTTYAudioRate
		for _, in := range interferers {
			local := in
			local.XDT = in.XDT - targetForward
			if local.XDT < -0.25 || local.XDT > frameDurationSeconds()+0.25 {
				continue
			}
			localInterferers = append(localInterferers, local)
		}
		if len(localInterferers) == 0 {
			continue
		}
		retroJob := target
		retroJob.retro = true
		retroJob.retroKey = key
		retroJob.interferers = localInterferers
		select {
		case s.retroJobs <- retroJob:
			s.retroSeen[key] = struct{}{}
		default:
			// The forward decoder has priority over retro work. A full queue simply
			// means this optional revisit is skipped for this pass.
		}
	}
}

func frameDurationSeconds() float64 {
	return float64(JTTYFrameSamples12k) / JTTYAudioRate
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func resampleLinear12(in []float32, from, to int) []float32 {
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
