package jtty

import (
	"context"
	"errors"
	"math"
	"math/cmplx"
	"sync"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/decoder"
	"github.com/BH2VSQ/jtty-go/internal/model"
)

var ErrInvalidWaveform = errors.New("invalid JTTY decode waveform")

// Decoder implements the JTTY PHY path using the WSJT-X 3.2.0-rc1 algorithmic
// reference: analytic 12 kHz -> 6 kHz conversion, sync search, payload tone
// correlations, TBCC/WAVA+CRC, and source-grammar validation.
type Decoder struct {
	Reference Reference
	Smin      float64
	mu        sync.RWMutex
	FTol      float64
}

// SetFrequencyTolerance updates the JTTY sync/search tolerance at runtime.
// The WSJT-X JTTY UI feeds this value into the wide-graph/JTTY decoder path;
// keeping it behind a lock prevents a settings change from racing a decode.
func (d *Decoder) SetFrequencyTolerance(hz float64) {
	if hz < 1 {
		hz = 1
	}
	if hz > 500 {
		hz = 500
	}
	d.mu.Lock()
	d.FTol = hz
	d.mu.Unlock()
}

func (d *Decoder) FrequencyTolerance() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.FTol
}

// SetMinSNR updates the final sync/SNR acceptance floor at runtime.
func (d *Decoder) SetMinSNR(snr float64) {
	if snr < -30 {
		snr = -30
	}
	if snr > 30 {
		snr = 30
	}
	d.mu.Lock()
	d.Smin = snr
	d.mu.Unlock()
}

func (d *Decoder) MinSNR() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Smin
}

func NewDecoder() *Decoder {
	return &Decoder{Reference: WSJTXReference, Smin: 4.6, FTol: 50.0}
}

var (
	sync192Once sync.Once
	sync192     []complex128
)

func cachedSyncWaveform(samplesPerSymbol int) []complex128 {
	if samplesPerSymbol == nSS {
		sync192Once.Do(func() { sync192 = GenerateSyncWaveform(nSS) })
		return sync192
	}
	return GenerateSyncWaveform(samplesPerSymbol)
}

const (
	decoderSampleRate = 12000
	fs6k              = 6000
	nSS               = 192 // 6000 / 31.25
	nFrameSymbols     = 59
	nFrameSamples6k   = nFrameSymbols * nSS
	nSyncSymbols      = 13
	nPayloadSymbols   = 46
	nSyncSamples6k    = nSyncSymbols * nSS
	nFFT              = 4096
)

// Decode decodes one 12 kHz int16-like float32 waveform already narrow enough
// to contain the JTTY signal. The receiver pipeline normally supplies samples
// resampled from the configured sound-card rate.
func (d *Decoder) Decode(ctx context.Context, job decoder.Job) ([]model.DecodeMessage, error) {
	msgs, err := d.DecodeMany(ctx, job, 1, false)
	if err != nil || len(msgs) == 0 {
		return msgs, err
	}
	return msgs[:1], nil
}

// DecodeMany implements the multi-candidate search used by WSJT-X's
// jtty_mdecode: an FFT-based sync surface, exclusion masks for multiple peaks,
// per-candidate peak-up, decode validation, and optional decoded-signal
// subtraction before a second sweep. The full active-message/retro state
// machine is kept above the PHY layer for the next receive-state milestone.
func (d *Decoder) DecodeMany(ctx context.Context, job decoder.Job, maxCandidates int, enableSubtraction bool) ([]model.DecodeMessage, error) {
	hits, err := d.DecodeManyDetailed(ctx, job, maxCandidates, enableSubtraction, nil)
	if err != nil {
		return nil, err
	}
	msgs := make([]model.DecodeMessage, 0, len(hits))
	for _, hit := range hits {
		msgs = append(msgs, hit.Message)
	}
	return msgs, nil
}

// KnownInterferer is the source-equivalent input used by jtty_mdecode's
// retro-re-sweep: a previously accepted, re-encoded full frame is subtracted
// from an older search window before the search is repeated.
type KnownInterferer struct {
	FrequencyHz float64
	XDT         float64
	Tones       [59]int
}

// DecodedFrame keeps the exact payload/tone sequence that produced a valid
// message. The normal public DecodeMany API exposes only model.DecodeMessage;
// the realtime receiver uses this richer form so subtraction can use the
// same error-corrected tone sequence that WSJT-X feeds to subtract_jtty.
type DecodedFrame struct {
	Message     model.DecodeMessage
	Payload     [PayloadBits]int
	Tones       [59]int
	FrequencyHz float64
	XDT         float64
}

// DecodeManyDetailed is the source-oriented multi-candidate decoder. known
// contains optional interferers already decoded in a newer window; they are
// subtracted before this window's candidate search.
func (d *Decoder) DecodeManyDetailed(ctx context.Context, job decoder.Job, maxCandidates int, enableSubtraction bool, known []KnownInterferer) ([]DecodedFrame, error) {
	return d.decodeManyDetailed(ctx, job, maxCandidates, enableSubtraction, known, false)
}

// DecodeWidebandManyDetailed runs the same PHY decoder over the complete
// JTTY passband instead of restricting the initial sync search to the current
// RX DF tolerance. The detector may still report spectral hints for display,
// but decode should not depend on the first four FFT peaks.
func (d *Decoder) DecodeWidebandManyDetailed(ctx context.Context, job decoder.Job, maxCandidates int, enableSubtraction bool, known []KnownInterferer) ([]DecodedFrame, error) {
	return d.decodeManyDetailed(ctx, job, maxCandidates, enableSubtraction, known, true)
}

type subtractWorkspace struct {
	camp   []complex128
	kernel []complex128
	ref    []complex128
	dphi   []float64
	pulse  []float64
	nfft   int
}

var (
	complexScratchPool sync.Pool
	floatScratchPool   sync.Pool
	subtractPool       sync.Pool
)

func acquireComplexScratch(n int) []complex128 {
	if n <= 0 {
		return nil
	}
	if v := complexScratchPool.Get(); v != nil {
		b := v.([]complex128)
		if cap(b) >= n {
			return b[:n]
		}
	}
	return make([]complex128, n)
}

func releaseComplexScratch(b []complex128) {
	if cap(b) == 0 || cap(b) > 1<<17 {
		return
	}
	complexScratchPool.Put(b[:0])
}

func acquireFloatScratch(n int) []float64 {
	if n <= 0 {
		return nil
	}
	if v := floatScratchPool.Get(); v != nil {
		b := v.([]float64)
		if cap(b) >= n {
			return b[:n]
		}
	}
	return make([]float64, n)
}

func releaseFloatScratch(b []float64) {
	if cap(b) == 0 || cap(b) > 1<<15 {
		return
	}
	floatScratchPool.Put(b[:0])
}

func acquireSubtractWorkspace(nfft, refLen int) *subtractWorkspace {
	var ws *subtractWorkspace
	if v := subtractPool.Get(); v != nil {
		ws = v.(*subtractWorkspace)
	}
	if ws == nil {
		ws = &subtractWorkspace{}
	}
	if cap(ws.camp) < nfft {
		ws.camp = make([]complex128, nfft)
	}
	if cap(ws.kernel) < nfft {
		ws.kernel = make([]complex128, nfft)
		ws.nfft = 0
	}
	if cap(ws.ref) < refLen {
		ws.ref = make([]complex128, refLen)
	}
	if cap(ws.dphi) < refLen+2*nSS {
		ws.dphi = make([]float64, refLen+2*nSS)
	}
	if cap(ws.pulse) < 3*nSS {
		ws.pulse = make([]float64, 3*nSS)
	}
	ws.camp = ws.camp[:nfft]
	ws.kernel = ws.kernel[:nfft]
	ws.ref = ws.ref[:refLen]
	ws.dphi = ws.dphi[:refLen+2*nSS]
	ws.pulse = ws.pulse[:3*nSS]
	if ws.nfft != nfft {
		clear(ws.kernel)
		nfilt := 2 * nSS
		half := nfilt / 2
		sumW := 0.0
		for k := -half; k <= half; k++ {
			w := math.Cos(math.Pi * float64(k) / float64(nfilt))
			w *= w
			sumW += w
		}
		if sumW > 0 {
			for k := -half; k <= half; k++ {
				w := math.Cos(math.Pi * float64(k) / float64(nfilt))
				w *= w / sumW
				idx := k
				if idx < 0 {
					idx += nfft
				}
				ws.kernel[idx] = complex(w, 0)
			}
		}
		fftInPlace(ws.kernel, false)
		ws.nfft = nfft
	}
	return ws
}

func releaseSubtractWorkspace(ws *subtractWorkspace) {
	if ws == nil || cap(ws.camp) > 1<<17 || cap(ws.kernel) > 1<<17 || cap(ws.ref) > 1<<17 {
		return
	}
	subtractPool.Put(ws)
}

func (d *Decoder) decodeManyDetailed(ctx context.Context, job decoder.Job, maxCandidates int, enableSubtraction bool, known []KnownInterferer, wideband bool) ([]DecodedFrame, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if len(job.Samples) < nFrameSamples6k*2 || (!wideband && job.FrequencyHz < 0) {
		return nil, ErrInvalidWaveform
	}
	nFFTBuffer := 1
	for nFFTBuffer < len(job.Samples) {
		nFFTBuffer <<= 1
	}
	fftBuffer := acquireComplexScratch(nFFTBuffer)
	defer releaseComplexScratch(fftBuffer)
	x6 := analytic6kInto(job.Samples, decoderSampleRate, fftBuffer)
	if len(x6) < nSyncSamples6k+nPayloadSymbols*nSS {
		return nil, ErrInvalidWaveform
	}
	if maxCandidates < 1 {
		maxCandidates = 1
	}
	if maxCandidates > 8 {
		maxCandidates = 8
	}
	ftol := d.FrequencyTolerance()
	f0 := job.FrequencyHz
	if wideband {
		// JTTY's source decoder searches approximately 200..2800 Hz. Keep
		// the wideband path independent from the user-selected RX DF so a
		// weak signal away from the current cursor is still discoverable.
		f0 = 1500
		ftol = 1300
	} else {
		if ftol <= 0 {
			ftol = 50
		}
		if f0 <= 0 {
			f0 = 1500
		}
	}

	// This follows jtty_mdecode's two-phase channel ordering instead of
	// globally sorting candidates from all channels. The QSO channel gets the
	// first claim; contest side channels then search the post-subtraction
	// residual. This makes the Go path deterministic under QRM.
	const (
		nfa            = 200.0
		nfb            = 2800.0
		defaultSideMax = 2
	)
	nfzBins := int(math.Round(10.0 / (float64(fs6k) / nFFT)))
	if nfzBins < 1 {
		nfzBins = 1
	}
	primaryMax := int(math.Round(ftol / (float64(nfzBins) * (float64(fs6k) / nFFT))))
	primaryMax = maxInt(2, minInt(8, primaryMax))
	primaryMax = minInt(primaryMax, maxCandidates)

	residual := x6
	for _, interferer := range known {
		subtractKnownSignal(residual, interferer.Tones[:], interferer.FrequencyHz, interferer.XDT)
	}
	results := make([]DecodedFrame, 0, maxCandidates)
	primaryOK := make([]acceptedFrame, 0, primaryMax)
	workband := acquireComplexScratch(len(x6))
	defer releaseComplexScratch(workband)

	// Phase A: channel 0. A second pass is only useful after at least one
	// successful signal has been subtracted, matching jtty_mdecode's any_subtracted gate.
	for pass := 0; pass < 2 && len(results) < maxCandidates; pass++ {
		cands := searchSyncCandidatesContext(ctx, residual, f0, ftol, nfa, nfb, primaryMax)
		if len(cands) == 0 {
			break
		}
		accepted := false
		for _, c := range cands {
			f1, xdt, peak := peakUpContext(ctx, residual, c.dt, c.freqHz, workband)
			if peak <= 0 {
				f1, xdt = c.freqHz, c.dt
			}
			hit, ok := d.tryCandidate(ctx, residual, job, f1, xdt, d.MinSNR(), 7, workband)
			if !ok {
				continue
			}
			if isDuplicateDetailedResult(results, hit.message.Message, hit.f1, hit.xdt) {
				continue
			}
			results = append(results, DecodedFrame{Message: hit.message, Payload: hit.payload, Tones: hit.tones, FrequencyHz: hit.f1, XDT: hit.xdt})
			primaryOK = append(primaryOK, acceptedFrame{frequencyHz: hit.f1, xdt: hit.xdt})
			accepted = true
			if enableSubtraction {
				subtractKnownSignal(residual, hit.tones[:], hit.f1, hit.xdt)
			}
			if len(results) >= maxCandidates {
				break
			}
		}
		if !accepted || !enableSubtraction {
			break
		}
	}

	if wideband {
		return results, nil
	}

	// Phase B: the two source-defined side channels. Run both channels in each
	// pass so a successful signal on one side is subtracted before the next side
	// search. The primary channel's successful neighborhoods are explicitly
	// excluded as in jtty_mdecode's private channel-0 mask.
	for pass := 0; pass < 2 && len(results) < maxCandidates; pass++ {
		anySubtracted := false
		for _, center := range []float64{1350, 1650} {
			cands := searchSyncCandidatesContext(ctx, residual, center, 150, nfa, nfb, minInt(defaultSideMax, maxCandidates-len(results)))
			for _, c := range cands {
				if nearAcceptedPrimary(primaryOK, c.freqHz, c.dt) {
					continue
				}
				hit, ok := d.tryCandidate(ctx, residual, job, c.freqHz, c.dt, 5.0, 9, workband)
				if !ok {
					continue
				}
				if isDuplicateDetailedResult(results, hit.message.Message, hit.f1, hit.xdt) {
					continue
				}
				results = append(results, DecodedFrame{Message: hit.message, Payload: hit.payload, Tones: hit.tones, FrequencyHz: hit.f1, XDT: hit.xdt})
				if enableSubtraction {
					subtractKnownSignal(residual, hit.tones[:], hit.f1, hit.xdt)
					anySubtracted = true
				}
				if len(results) >= maxCandidates {
					break
				}
			}
			if len(results) >= maxCandidates {
				break
			}
		}
		if !anySubtracted || !enableSubtraction {
			break
		}
	}
	return results, nil
}

type acceptedFrame struct {
	frequencyHz float64
	xdt         float64
}

type candidateHit struct {
	message model.DecodeMessage
	payload [PayloadBits]int
	tones   [59]int
	f1      float64
	xdt     float64
}

func (d *Decoder) tryCandidate(ctx context.Context, samples []complex128, job decoder.Job, f1, xdt, minSNR float64, minSync int, work []complex128) (candidateHit, bool) {
	var zero candidateHit
	if ctx != nil {
		select {
		case <-ctx.Done():
			return zero, false
		default:
		}
	}
	snrDB, _, nsync := validateSyncContext(ctx, samples, f1, xdt, work)
	if nsync < minSync || snrDB < minSNR {
		return zero, false
	}
	baseband := work
	shiftFrequencyInto(baseband, samples, -f1, fs6k)
	payloadStart := int(math.Round(xdt*float64(fs6k))) + nSyncSamples6k
	corr, half := CorrelatePayloadSymbols(baseband, payloadStart, nSS)
	payload, ok := DecodePayloadContext(ctx, corr, half)
	if !ok {
		return zero, false
	}
	var bitsFrame PackedBits34
	copy(bitsFrame[:], payload[:])
	packed := PackedFrame{Bits: bitsToPacked(bitsFrame)}
	msg, _, eom, sourceValid := UnpackMessage([]PackedFrame{packed})
	if !sourceValid {
		return zero, false
	}
	msg = displayJTTYText(msg)
	message := decomposeDecodeMessage(msg, job, f1, xdt, snrDB, eom)
	return candidateHit{message: message, payload: payload, tones: EncodeFrame(payload), f1: f1, xdt: xdt}, true
}

func nearAcceptedPrimary(list []acceptedFrame, freqHz, xdt float64) bool {
	for _, v := range list {
		if math.Abs(v.frequencyHz-freqHz) <= 10.5 && math.Abs(v.xdt-xdt) <= 0.016 {
			return true
		}
	}
	return false
}

func isDuplicateDetailedResult(results []DecodedFrame, message string, freqHz, dt float64) bool {
	for _, r := range results {
		if r.Message.Message != message {
			continue
		}
		if math.Abs(r.FrequencyHz-freqHz) < 12.0 && math.Abs(r.XDT-dt) < 0.032 {
			return true
		}
	}
	return false
}

// PackedBits34 is a tiny alias used only to make the 34-bit payload conversion explicit.
type PackedBits34 [PayloadBits]int

func bitsToPacked(bits [PayloadBits]int) uint64 {
	var v uint64
	for i, b := range bits {
		if b != 0 {
			v |= uint64(1) << uint(PayloadBits-1-i)
		}
	}
	return v << 0
}

func decomposeDecodeMessage(msg string, job decoder.Job, f1, dt, snrDB float64, eom bool) model.DecodeMessage {
	now := time.Now().UTC()
	signalUTC := now
	if !job.Timestamp.IsZero() {
		signalUTC = job.Timestamp.UTC().Add(time.Duration(dt * float64(time.Second)))
	}
	return model.DecodeMessage{
		SignalUTC:     signalUTC,
		ReceivedUTC:   now,
		SNR:           int(math.Round(snrDB)),
		DT:            dt,
		FrequencyHz:   int(math.Round(f1)),
		Message:       msg,
		Callsigns:     ExtractCallsigns(msg),
		Confidence:    snrDB,
		JTTYLastFrame: eom,
	}
}

func analytic6k(in []float32, sampleRate int) []complex128 {
	n := 1
	for n < len(in) {
		n <<= 1
	}
	buf := make([]complex128, n)
	out := analytic6kInto(in, sampleRate, buf)
	if out == nil {
		return nil
	}
	result := make([]complex128, len(out))
	copy(result, out)
	return result
}

func analytic6kInto(in []float32, sampleRate int, buf []complex128) []complex128 {
	if sampleRate != decoderSampleRate || len(in) == 0 {
		return nil
	}
	n := 1
	for n < len(in) {
		n <<= 1
	}
	if cap(buf) < n {
		return nil
	}
	buf = buf[:n]
	clear(buf)
	for i, v := range in {
		buf[i] = complex(float64(v), 0)
	}
	fftInPlace(buf, false)
	halfN := n / 2
	if halfN < 4 {
		return nil
	}
	for i := halfN/2 + 1; i < halfN; i++ {
		buf[i] = 0
	}
	buf[0] *= 0.5
	fftInPlace(buf[:halfN], true)
	outLen := len(in) / 2
	if outLen <= 0 || outLen > halfN {
		return nil
	}
	return buf[:outLen]
}

func fftInPlace(a []complex128, inverse bool) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		if inverse {
			angle = -angle
		}
		wLen := cmplx.Exp(complex(0, angle))
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			half := length / 2
			for j := 0; j < half; j++ {
				u := a[i+j]
				v := a[i+j+half] * w
				a[i+j] = u + v
				a[i+j+half] = u - v
				w *= wLen
			}
		}
	}
	if inverse {
		invN := 1 / float64(n)
		for i := range a {
			a[i] *= complex(invN, 0)
		}
	}
}

func shiftFrequency(in []complex128, hz, sampleRate float64) []complex128 {
	out := make([]complex128, len(in))
	shiftFrequencyInto(out, in, hz, sampleRate)
	return out
}

func shiftFrequencyInto(out, in []complex128, hz, sampleRate float64) {
	if len(out) < len(in) {
		panic("shiftFrequencyInto: destination too small")
	}
	out = out[:len(in)]
	if len(in) == 0 {
		return
	}
	if sampleRate <= 0 || hz == 0 {
		copy(out, in)
		return
	}
	phaseStep := 2 * math.Pi * hz / sampleRate
	step := complex(math.Cos(phaseStep), math.Sin(phaseStep))
	rot := complex(1.0, 0.0)
	for i, v := range in {
		out[i] = v * rot
		rot *= step
		if (i & 4095) == 4095 {
			m := cmplx.Abs(rot)
			if m > 0 {
				rot /= complex(m, 0)
			}
		}
	}
}
func syncCorrelationAt(x []complex128, sync []complex128, start int, freqBin float64, sampleRate float64) float64 {
	if start < 0 || start+len(sync) > len(x) {
		return 0
	}
	phaseStep := 2 * math.Pi * freqBin / sampleRate
	phase := 0.0
	var z complex128
	for i := range sync {
		rot := complex(math.Cos(-phase), math.Sin(-phase))
		z += cmplx.Conj(sync[i]) * x[start+i] * rot
		phase += phaseStep
	}
	return real(z * cmplx.Conj(z))
}

type syncCandidate struct {
	freqHz float64
	dt     float64
	value  float64
}

func searchSync(x []complex128, f0, ftol float64) (fbest, dtbest, snrDB, syncPower float64, ok bool) {
	cands := searchSyncCandidates(x, f0, ftol, 200, 2800, 1)
	if len(cands) == 0 {
		return 0, 0, -99, 0, false
	}
	c := cands[0]
	work := make([]complex128, len(x))
	fbest, dtbest, _ = peakUp(x, c.dt, c.freqHz, work)
	if fbest == 0 && dtbest == 0 {
		fbest, dtbest = c.freqHz, c.dt
	}
	snrDB, syncPower, nsync := validateSync(x, fbest, dtbest, work)
	return fbest, dtbest, snrDB, syncPower, nsync > 6
}

func searchSyncCandidates(x []complex128, f0, ftol, nfa, nfb float64, maxCandidates int) []syncCandidate {
	return searchSyncCandidatesContext(context.Background(), x, f0, ftol, nfa, nfb, maxCandidates)
}

func searchSyncCandidatesContext(ctx context.Context, x []complex128, f0, ftol, nfa, nfb float64, maxCandidates int) []syncCandidate {
	if len(x) < nSyncSamples6k || maxCandidates <= 0 {
		return nil
	}
	const minBin = 3
	const maxBin = nFFT/2 - 2
	df := float64(fs6k) / nFFT
	low := math.Max(nfa, f0-ftol)
	high := math.Min(nfb, f0+ftol)
	ja := int(math.Max(minBin, math.Floor(low/df)))
	jb := int(math.Min(maxBin, math.Ceil(high/df)))
	if ja > jb {
		return nil
	}
	ntStep := nFrameSamples6k / 4
	// The coarse sync surface uses a 4 ms time grid. peakUp() then refines
	// each accepted candidate over a +/-4 ms interval, cutting FFT work
	// substantially while preserving the live timing search range.
	const timeStepSamples = 96
	steps := ntStep/timeStepSamples + 1
	freqBins := jb - ja + 1
	surface := make([]float64, steps*freqBins)
	fftBuf := make([]complex128, nFFT)
	sync := cachedSyncWaveform(nSS)
	for istep, i0 := 0, 0; i0 <= ntStep; istep, i0 = istep+1, i0+timeStepSamples {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}
		for i := range fftBuf {
			fftBuf[i] = 0
		}
		for i := 0; i < nSyncSamples6k; i++ {
			fftBuf[i] = cmplx.Conj(sync[i]) * x[i0+i]
		}
		fftInPlace(fftBuf, false)
		for j := ja; j <= jb; j++ {
			p0 := cmplx.Abs(fftBuf[j-2])
			p1 := cmplx.Abs(fftBuf[j-1])
			p2 := cmplx.Abs(fftBuf[j])
			p3 := cmplx.Abs(fftBuf[j+1])
			p4 := cmplx.Abs(fftBuf[j+2])
			surface[istep*freqBins+j-ja] = p0*p0 + 2*p1*p1 + 3*p2*p2 + 2*p3*p3 + p4*p4
		}
	}

	nfz := int(math.Round(10.0 / df))
	ntz := int(math.Round(0.016 * fs6k / float64(timeStepSamples)))
	out := make([]syncCandidate, 0, maxCandidates)
	for len(out) < maxCandidates {
		best := -1
		bestValue := 0.0
		for i, v := range surface {
			if v > bestValue {
				bestValue = v
				best = i
			}
		}
		if best < 0 || bestValue <= 0 {
			break
		}
		step := best / freqBins
		binIndex := best - step*freqBins
		freqBin := ja + binIndex
		out = append(out, syncCandidate{freqHz: float64(freqBin) * df, dt: float64(step*timeStepSamples) / fs6k, value: bestValue})
		stepLo, stepHi := maxInt(0, step-ntz), minInt(steps-1, step+ntz)
		binLo, binHi := maxInt(0, binIndex-nfz), minInt(freqBins-1, binIndex+nfz)
		for s := stepLo; s <= stepHi; s++ {
			for b := binLo; b <= binHi; b++ {
				surface[s*freqBins+b] = 0
			}
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func validateSync(x []complex128, f1, xdt float64, work []complex128) (snrDB, signalPower float64, nsync int) {
	return validateSyncContext(context.Background(), x, f1, xdt, work)
}

func validateSyncContext(ctx context.Context, x []complex128, f1, xdt float64, work []complex128) (snrDB, signalPower float64, nsync int) {
	if len(work) < len(x) {
		work = make([]complex128, len(x))
	} else {
		work = work[:len(x)]
	}
	shiftFrequencyInto(work, x, -f1, fs6k)
	y := work
	refs := cachedReference(nSS)
	sumSignal, sumAll := 0.0, 0.0
	start0 := int(math.Round(xdt * fs6k))
	for j, tone := range SyncTones {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return -99.9, 0, 0
			default:
			}
		}
		first := start0 + j*nSS
		if first < 0 || first+nSS > len(y) {
			break
		}
		powers := [4]float64{}
		for t := 0; t < 4; t++ {
			var z complex128
			for i := 0; i < nSS; i++ {
				z += cmplx.Conj(refs[t][i]) * y[first+i]
			}
			powers[t] = cmplx.Abs(z) * cmplx.Abs(z)
		}
		best := 0
		for t := 1; t < 4; t++ {
			if powers[t] > powers[best] {
				best = t
			}
		}
		if best == tone {
			nsync++
		}
		sumSignal += powers[tone]
		sumAll += powers[0] + powers[1] + powers[2] + powers[3]
	}
	noise := (sumAll - sumSignal) / 3
	if noise > 0 && sumSignal > 0 {
		snrDB = 10 * math.Log10(sumSignal/noise)
	} else {
		snrDB = -99.9
	}
	return snrDB, sumSignal, nsync
}

func peakUp(x []complex128, xdt0, f0 float64, work []complex128) (f1, xdt float64, pmax float64) {
	return peakUpContext(context.Background(), x, xdt0, f0, work)
}

func peakUpContext(ctx context.Context, x []complex128, xdt0, f0 float64, work []complex128) (f1, xdt float64, pmax float64) {
	const hop = 4
	npsync := nSyncSamples6k
	if len(x) < npsync {
		return f0, xdt0, 0
	}
	dt := 1.0 / float64(fs6k)
	ia := int(math.Max(0, math.Round((xdt0-0.008)/dt)))
	ib := int(math.Min(float64(len(x)-npsync), math.Round((xdt0+0.008)/dt)))
	if ia > ib {
		return f0, xdt0, 0
	}
	sync := cachedSyncWaveform(nSS)
	var qstep [13]complex128
	for i := 0; i < 13; i++ {
		istart := i * nSS
		qstep[i] = sync[istart+hop] * cmplx.Conj(sync[istart])
	}
	var zbest [13]complex128
	for idf := -5; idf <= 5; idf++ {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return f0, xdt0, 0
			default:
			}
		}
		ftrial := f0 - 0.5*float64(idf)
		if len(work) < len(x) {
			work = make([]complex128, len(x))
		} else {
			work = work[:len(x)]
		}
		shiftFrequencyInto(work, x, -ftrial, fs6k)
		y := work
		var zcur [13]complex128
		for i := 0; i < 13; i++ {
			start := i * nSS
			var z complex128
			for j := 0; j < nSS; j++ {
				z += cmplx.Conj(sync[start+j]) * y[ia+start+j]
			}
			zcur[i] = z
		}
		for i0 := ia; i0 <= ib; i0 += hop {
			p := 0.0
			for i := 0; i < 13; i++ {
				p += cmplx.Abs(zcur[i]) * cmplx.Abs(zcur[i])
			}
			if p > pmax {
				pmax, f1, xdt, zbest = p, ftrial, float64(i0)*dt, zcur
			}
			if i0+hop > ib {
				break
			}
			for i := 0; i < 13; i++ {
				istart := i * nSS
				var oldZ, newZ complex128
				for r := 0; r < hop; r++ {
					oldZ += cmplx.Conj(sync[istart+r]) * y[i0+istart+r]
					newZ += cmplx.Conj(sync[istart+nSS-hop+r]) * y[i0+istart+nSS+r]
				}
				zcur[i] = qstep[i]*(zcur[i]-oldZ) + newZ
			}
		}
	}
	if pmax <= 0 {
		return f0, xdt0, 0
	}
	var uw [13]float64
	for i := 0; i < 13; i++ {
		uw[i] = math.Atan2(imag(zbest[i]), real(zbest[i]))
	}
	for i := 1; i < 13; i++ {
		d := uw[i] - uw[i-1]
		for d > math.Pi {
			d -= 2 * math.Pi
		}
		for d < -math.Pi {
			d += 2 * math.Pi
		}
		uw[i] = uw[i-1] + d
	}
	ym := 0.0
	for i := range uw {
		ym += uw[i]
	}
	ym /= 13
	sxy, sxx := 0.0, 0.0
	for i := 0; i < 13; i++ {
		dx := float64(i) - 6
		sxy += dx * (uw[i] - ym)
		sxx += dx * dx
	}
	slope := sxy / sxx
	intercept := ym - slope*6
	residRMS := 0.0
	for i := 0; i < 13; i++ {
		r := uw[i] - (slope*float64(i) + intercept)
		residRMS += r * r
	}
	residRMS = math.Sqrt(residRMS / 13)
	dfHz := slope / (2 * math.Pi * (float64(nSS) / fs6k))
	if residRMS < 1 && math.Abs(dfHz) <= 0.5 {
		var ztot complex128
		for i := 0; i < 13; i++ {
			ztot += zbest[i] * complex(math.Cos(-slope*float64(i)), math.Sin(-slope*float64(i)))
		}
		f1 += dfHz
		pmax = cmplx.Abs(ztot) * cmplx.Abs(ztot)
	}
	return f1, xdt, pmax
}

func subtractKnownSignal(x []complex128, tones []int, f1, xdt float64) {
	if len(x) == 0 || len(tones) == 0 {
		return
	}
	refLen := len(tones) * nSS
	start := int(math.Round(xdt * fs6k))
	if start >= len(x) {
		return
	}
	first, last := 0, refLen
	if start < 0 {
		first = -start
		start = 0
	}
	if start+last > len(x) {
		last = len(x) - start
	}
	if last <= first {
		return
	}
	nfilt := 2 * nSS
	nfft := 1
	need := last + nfilt + 1
	for nfft < need {
		nfft <<= 1
	}
	ws := acquireSubtractWorkspace(nfft, refLen)
	defer releaseSubtractWorkspace(ws)
	clear(ws.camp)
	generateJTTYComplexWaveformInto(ws.ref, ws.dphi, ws.pulse, tones, nSS, 2.0, fs6k, f1)
	for i := first; i < last; i++ {
		j := start + i
		ws.camp[i] = x[j] * cmplx.Conj(ws.ref[i])
	}
	fftInPlace(ws.camp, false)
	for i := range ws.camp {
		ws.camp[i] *= ws.kernel[i]
	}
	fftInPlace(ws.camp, true)
	for i := first; i < last; i++ {
		j := start + i
		x[j] -= ws.camp[i] * ws.ref[i]
	}
}
