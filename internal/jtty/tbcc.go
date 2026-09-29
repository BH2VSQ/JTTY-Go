package jtty

import (
	"context"
	"math"
	"math/cmplx"
	"sync"
)

const (
	PayloadBits            = 34
	OuterCheckBits         = 12
	InformationBits        = 46
	MemoryNu               = 9
	StateCount             = 512
	Generator0      uint32 = 0o1167
	Generator1      uint32 = 0o1545
	OuterPolynomial        = 0x80F
	MaxHypotheses          = 4
)

var SyncTones = [13]int{0, 2, 2, 3, 0, 0, 3, 2, 1, 3, 1, 2, 0}

func parityBit(v uint32) int {
	v ^= v >> 16
	v ^= v >> 8
	v ^= v >> 4
	return int((0x6996 >> (v & 0xF)) & 1)
}

func transition(state, input int) (next, tone int) {
	reg := ((state << 1) | input) & 0x3FF
	b0 := parityBit(uint32(reg) & Generator0)
	b1 := parityBit(uint32(reg) & Generator1)
	switch 2*b0 + b1 {
	case 0:
		tone = 0
	case 1:
		tone = 1
	case 3:
		tone = 2
	case 2:
		tone = 3
	}
	return reg & 0x1FF, tone
}

func crc12(payload [PayloadBits]int) [InformationBits]int {
	out := [InformationBits]int{}
	copy(out[:PayloadBits], payload[:])
	reg := 0
	for i := 0; i < PayloadBits; i++ {
		reg ^= out[i] << (OuterCheckBits - 1)
		if reg&0x800 != 0 {
			reg = ((reg << 1) ^ OuterPolynomial) & 0xFFF
		} else {
			reg = (reg << 1) & 0xFFF
		}
	}
	for i := 0; i < OuterCheckBits; i++ {
		out[PayloadBits+i] = (reg >> (OuterCheckBits - 1 - i)) & 1
	}
	return out
}

func EncodePayload(payload [PayloadBits]int) [InformationBits]int {
	info := crc12(payload)
	var out [InformationBits]int
	state := 0
	for t := 0; t < MemoryNu; t++ {
		bit := info[InformationBits-MemoryNu+t]
		state = ((state << 1) | bit) & 0x1FF
	}
	for t := 0; t < InformationBits; t++ {
		bit := info[t]
		reg := ((state << 1) | bit) & 0x3FF
		b0 := parityBit(uint32(reg) & Generator0)
		b1 := parityBit(uint32(reg) & Generator1)
		out[t] = 2*b0 + b1
		state = reg & 0x1FF
	}
	// Convert binary pair values to Gray-tone indices exactly as WSJT-X.
	for i, v := range out {
		switch v {
		case 0:
			out[i] = 0
		case 1:
			out[i] = 1
		case 3:
			out[i] = 2
		case 2:
			out[i] = 3
		}
	}
	return out
}

// EncodeFrame returns the complete 59-tone JTTY frame: 13 sync + 46 coded tones.
func EncodeFrame(payload [PayloadBits]int) [59]int {
	coded := EncodePayload(payload)
	var tones [59]int
	copy(tones[:13], SyncTones[:])
	copy(tones[13:], coded[:])
	return tones
}

type ComplexCorrelations [4][InformationBits]complex128
type HalfCorrelations [4][InformationBits]complex128

var (
	reference192Once sync.Once
	reference192     [4][]complex128
)

func cachedReference(samplesPerSymbol int) [4][]complex128 {
	if samplesPerSymbol == nSS {
		reference192Once.Do(func() { reference192 = buildReference(nSS) })
		return reference192
	}
	return buildReference(samplesPerSymbol)
}

func buildReference(samplesPerSymbol int) [4][]complex128 {
	refs := [4][]complex128{}
	for tone := 0; tone < 4; tone++ {
		refs[tone] = make([]complex128, samplesPerSymbol)
		phase := 0.0
		step := 2 * math.Pi * float64(tone) / float64(samplesPerSymbol)
		for i := 0; i < samplesPerSymbol; i++ {
			refs[tone][i] = complex(math.Cos(phase), math.Sin(phase))
			phase += step
		}
	}
	return refs
}

// CorrelatePayloadSymbols mirrors jtty_payload_correlators.f90.
func CorrelatePayloadSymbols(samples []complex128, payloadStart, samplesPerSymbol int) (ComplexCorrelations, HalfCorrelations) {
	var full ComplexCorrelations
	var half HalfCorrelations
	if samplesPerSymbol <= 0 || samplesPerSymbol%2 != 0 || payloadStart < 0 || payloadStart >= len(samples) {
		return full, half
	}
	refs := cachedReference(samplesPerSymbol)
	available := (len(samples) - payloadStart) / samplesPerSymbol
	if available > InformationBits {
		available = InformationBits
	}
	h := samplesPerSymbol / 2
	for sym := 0; sym < available; sym++ {
		first := payloadStart + sym*samplesPerSymbol
		for tone := 0; tone < 4; tone++ {
			var fullZ complex128
			e := 0.0
			for seg := 0; seg < 2; seg++ {
				var z complex128
				start := seg * h
				for i := 0; i < h; i++ {
					z += cmplx.Conj(refs[tone][start+i]) * samples[first+start+i]
				}
				fullZ += z
				e += real(z * cmplx.Conj(z))
			}
			full[tone][sym] = fullZ
			half[tone][sym] = complex(math.Sqrt(math.Max(0, e)), 0)
		}
	}
	return full, half
}

type survivor struct {
	metric float64
	bits   uint64
	origin int
}

type wavaCandidate struct {
	metric float64
	bits   uint64
	origin int
}

func coherentEnergy(corr ComplexCorrelations, start int, blockLen int, toneSeq []int) float64 {
	var z complex128
	for i := 0; i < blockLen; i++ {
		z += corr[toneSeq[i]][start+i]
	}
	return real(z*cmplx.Conj(z)) / float64(blockLen)
}

func halfEnergy(corr HalfCorrelations, start int, blockLen int, toneSeq []int) float64 {
	var z float64
	for i := 0; i < blockLen; i++ {
		z += real(corr[toneSeq[i]][start+i])
	}
	return (z * z) / float64(blockLen)
}

func sequenceEnergyForBlock(corr ComplexCorrelations, start, length int) []float64 {
	count := 1 << (2 * length)
	out := make([]float64, count)
	for word := 0; word < count; word++ {
		var z complex128
		for off := 0; off < length; off++ {
			tone := (word >> uint(2*(length-off-1))) & 3
			z += corr[tone][start+off]
		}
		out[word] = real(z*cmplx.Conj(z)) / float64(length)
	}
	return out
}

func branchForBlock(startState int, bits []int) (endState int, tones []int) {
	state := startState
	tones = make([]int, len(bits))
	for i, b := range bits {
		ns, tone := transition(state, b)
		state = ns
		tones[i] = tone
	}
	return state, tones
}

func crcValid(bits [InformationBits]int) bool {
	reg := 0
	for i := 0; i < InformationBits; i++ {
		reg ^= bits[i] << (OuterCheckBits - 1)
		if reg&0x800 != 0 {
			reg = ((reg << 1) ^ OuterPolynomial) & 0xFFF
		} else {
			reg = (reg << 1) & 0xFFF
		}
	}
	return reg == 0
}

func reservedZero(bits [InformationBits]int) bool { return bits[32] == 0 }

func candidateIdentity(bits [InformationBits]int) string {
	b := make([]byte, (InformationBits+7)/8)
	for i, v := range bits {
		if v != 0 {
			b[i/8] |= 1 << uint(i%8)
		}
	}
	return string(b)
}

// candidateIdentityKey is the allocation-free representation used by the hot
// WAVA result deduplication path. The 46 information bits fit in uint64.
func candidateIdentityKey(bits uint64) uint64 {
	return bits
}

func packedBitsToArray(bits uint64) [InformationBits]int {
	var out [InformationBits]int
	for i := 0; i < InformationBits; i++ {
		out[i] = int((bits >> uint(i)) & 1)
	}
	return out
}

type survivorList struct {
	items [MaxHypotheses]survivor
	n     uint8
}

type blockTransitionTable struct {
	endState [StateCount][16]uint16
	toneWord [StateCount][16]uint16
}

var (
	blockTransitionOnce sync.Once
	blockTransitions    [5]blockTransitionTable
	blockBits           [5][16]uint64
)

func initBlockTransitions() {
	for _, length := range []int{1, 2, 4} {
		for word := 0; word < (1 << length); word++ {
			var packed uint64
			for i := 0; i < length; i++ {
				if (word>>uint(length-1-i))&1 != 0 {
					packed |= uint64(1) << uint(i)
				}
			}
			blockBits[length][word] = packed
		}
		count := 1 << length
		for state := 0; state < StateCount; state++ {
			for word := 0; word < count; word++ {
				st := state
				toneWord := 0
				for i := 0; i < length; i++ {
					bit := (word >> uint(length-1-i)) & 1
					var tone int
					st, tone = transition(st, bit)
					toneWord = toneWord*4 + tone
				}
				blockTransitions[length].endState[state][word] = uint16(st)
				blockTransitions[length].toneWord[state][word] = uint16(toneWord)
			}
		}
	}
}

func fillSequenceEnergyForBlock(corr ComplexCorrelations, start, length int, energies *[256]float64) int {
	if length <= 0 {
		return 0
	}
	if length > 4 {
		length = 4
	}
	count := 1 << (2 * length)
	for i := 0; i < count; i++ {
		energies[i] = 0
	}
	invLength := 1.0 / float64(length)
	for word := 0; word < count; word++ {
		var z complex128
		for off := 0; off < length; off++ {
			tone := (word >> uint(2*(length-off-1))) & 3
			z += corr[tone][start+off]
		}
		energies[word] = real(z*cmplx.Conj(z)) * invLength
	}
	return count
}

func insertSurvivorFast(dst *survivorList, cand survivor, width int, dedup bool) {
	if width < 1 {
		return
	}
	if width > MaxHypotheses {
		width = MaxHypotheses
	}
	n := int(dst.n)
	if dedup {
		for i := 0; i < n; i++ {
			if dst.items[i].origin == cand.origin && dst.items[i].bits == cand.bits {
				if cand.metric > dst.items[i].metric {
					dst.items[i] = cand
				}
				return
			}
		}
	}
	pos := n
	for i := 0; i < n; i++ {
		if cand.metric >= dst.items[i].metric {
			pos = i
			break
		}
	}
	if pos >= width && n >= width {
		return
	}
	if n < width {
		n++
	}
	if pos >= width {
		pos = width - 1
	}
	for i := n - 1; i > pos; i-- {
		dst.items[i] = dst.items[i-1]
	}
	dst.items[pos] = cand
	dst.n = uint8(n)
}

func decodeRung(corr ComplexCorrelations, coherentLength, wraps, perStateWidth int) [MaxHypotheses][InformationBits]int {
	out, _ := decodeRungContext(context.Background(), corr, coherentLength, wraps, perStateWidth)
	return out
}

func decodeRungContext(ctx context.Context, corr ComplexCorrelations, coherentLength, wraps, perStateWidth int) ([MaxHypotheses][InformationBits]int, bool) {
	if coherentLength != 1 && coherentLength != 2 && coherentLength != 4 {
		return [MaxHypotheses][InformationBits]int{}, true
	}
	if wraps < 1 {
		wraps = 1
	}
	if perStateWidth < 1 {
		perStateWidth = 1
	}
	if perStateWidth > MaxHypotheses {
		perStateWidth = MaxHypotheses
	}
	blockTransitionOnce.Do(initBlockTransitions)

	var prev [StateCount]survivorList
	for state := 0; state < StateCount; state++ {
		prev[state].n = 1
		prev[state].items[0] = survivor{metric: 0, origin: state}
	}
	blockCount := (InformationBits + coherentLength - 1) / coherentLength

	var energies [256]float64
	for wrap := 0; wrap < wraps; wrap++ {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return [MaxHypotheses][InformationBits]int{}, false
			default:
			}
		}
		for state := 0; state < StateCount; state++ {
			n := int(prev[state].n)
			for i := 0; i < n; i++ {
				prev[state].items[i].origin = state
				prev[state].items[i].bits = 0
			}
		}

		for block := 0; block < blockCount; block++ {
			if ctx != nil {
				select {
				case <-ctx.Done():
					return [MaxHypotheses][InformationBits]int{}, false
				default:
				}
			}
			start := block * coherentLength
			length := coherentLength
			if start+length > InformationBits {
				length = InformationBits - start
			}
			fillSequenceEnergyForBlock(corr, start, length, &energies)
			var current [StateCount]survivorList
			table := &blockTransitions[length]
			count := 1 << length
			dedup := block == 0
			for state := 0; state < StateCount; state++ {
				pv := &prev[state]
				for pi := 0; pi < int(pv.n); pi++ {
					sv := pv.items[pi]
					for word := 0; word < count; word++ {
						endState := int(table.endState[state][word])
						toneWord := int(table.toneWord[state][word])
						bits := sv.bits | (blockBits[length][word] << uint(start))
						cand := survivor{metric: sv.metric + energies[toneWord], origin: sv.origin, bits: bits}
						insertSurvivorFast(&current[endState], cand, perStateWidth, dedup)
					}
				}
			}
			prev = current
		}
	}

	// Final WAVA survivor deduplication uses a fixed open-addressing table.
	// At most StateCount*MaxHypotheses entries can reach this stage, so a 4096
	// slot table keeps load factor <= 0.5 without allocating a Go map per rung.
	var table [4096]wavaCandidate
	for i := range table {
		table[i].origin = -1
	}
	const tableMask = len(table) - 1
	for state := 0; state < StateCount; state++ {
		pv := &prev[state]
		for i := 0; i < int(pv.n); i++ {
			sv := pv.items[i]
			if sv.origin != state {
				continue
			}
			key := candidateIdentityKey(sv.bits)
			idx := int((key ^ (key >> 32) ^ (key >> 16)) & uint64(tableMask))
			for {
				slot := &table[idx]
				if slot.origin < 0 {
					*slot = wavaCandidate{metric: sv.metric, bits: sv.bits, origin: state}
					break
				}
				if slot.bits == key {
					if sv.metric > slot.metric || (sv.metric == slot.metric && state < slot.origin) {
						slot.metric, slot.origin = sv.metric, state
					}
					break
				}
				idx = (idx + 1) & tableMask
			}
		}
	}

	var best [MaxHypotheses]wavaCandidate
	bestN := 0
	for i := range table {
		candidate := table[i]
		if candidate.origin < 0 {
			continue
		}
		pos := bestN
		for j := 0; j < bestN; j++ {
			if candidate.metric > best[j].metric || (candidate.metric == best[j].metric && candidate.origin < best[j].origin) {
				pos = j
				break
			}
		}
		if bestN < MaxHypotheses {
			bestN++
		}
		if pos >= bestN {
			continue
		}
		for j := bestN - 1; j > pos; j-- {
			best[j] = best[j-1]
		}
		best[pos] = candidate
	}

	var out [MaxHypotheses][InformationBits]int
	for i := 0; i < bestN; i++ {
		out[i] = packedBitsToArray(best[i].bits)
	}
	return out, true
}

// DecodePayload is the non-cancellable compatibility wrapper. The realtime
// receiver uses DecodePayloadContext so an in-flight optional decode can stop
// promptly when the capture pipeline is closed.
func DecodePayload(corr ComplexCorrelations, half HalfCorrelations) ([PayloadBits]int, bool) {
	return DecodePayloadContext(context.Background(), corr, half)
}

func DecodePayloadContext(ctx context.Context, corr ComplexCorrelations, half HalfCorrelations) ([PayloadBits]int, bool) {
	for _, L := range []int{1, 2, 4} {
		candidates, ok := decodeRungContext(ctx, corr, L, 2, 4)
		if !ok {
			return [PayloadBits]int{}, false
		}
		for i := 0; i < MaxHypotheses; i++ {
			if !crcValid(candidates[i]) || !reservedZero(candidates[i]) {
				continue
			}
			var payload [PayloadBits]int
			copy(payload[:], candidates[i][:PayloadBits])
			allZero := true
			for _, v := range payload {
				if v != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				continue
			}
			return payload, true
		}
	}
	full := ComplexCorrelations{}
	for t := 0; t < InformationBits; t++ {
		for tone := 0; tone < 4; tone++ {
			full[tone][t] = half[tone][t]
		}
	}
	candidates, ok := decodeRungContext(ctx, full, 1, 2, 4)
	if !ok {
		return [PayloadBits]int{}, false
	}
	for i := 0; i < MaxHypotheses; i++ {
		if !crcValid(candidates[i]) || !reservedZero(candidates[i]) {
			continue
		}
		var payload [PayloadBits]int
		copy(payload[:], candidates[i][:PayloadBits])
		allZero := true
		for _, v := range payload {
			if v != 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			return payload, true
		}
	}
	return [PayloadBits]int{}, false
}
