package audio

import "sync/atomic"

// RingBuffer is a single-writer/single-reader audio ring. When the reader falls
// behind, the oldest samples are discarded rather than blocking the realtime
// audio path. This is intentional: stale audio is less useful than current audio.
type RingBuffer struct {
	buf     []float32
	r       atomic.Uint64
	w       atomic.Uint64
	dropped atomic.Uint64
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{buf: make([]float32, capacity)}
}

func (rb *RingBuffer) Len() int {
	w, r := rb.w.Load(), rb.r.Load()
	return int(w - r)
}

func (rb *RingBuffer) Cap() int        { return len(rb.buf) }
func (rb *RingBuffer) Dropped() uint64 { return rb.dropped.Load() }

func (rb *RingBuffer) Write(src []float32) int {
	if len(src) == 0 {
		return 0
	}
	w := rb.w.Load()
	r := rb.r.Load()
	capN := uint64(len(rb.buf))
	if uint64(len(src)) > capN {
		droppedFromWrite := uint64(len(src)) - capN
		rb.dropped.Add(droppedFromWrite)
		src = src[len(src)-int(capN):]
	}
	need := uint64(len(src))
	used := w - r
	if used+need > capN {
		drop := used + need - capN
		r += drop
		rb.r.Store(r)
		rb.dropped.Add(drop)
	}
	for i, v := range src {
		rb.buf[int((w+uint64(i))%capN)] = v
	}
	rb.w.Store(w + need)
	return len(src)
}

func (rb *RingBuffer) Read(dst []float32) int {
	if len(dst) == 0 {
		return 0
	}
	r := rb.r.Load()
	w := rb.w.Load()
	available := int(w - r)
	if available <= 0 {
		return 0
	}
	n := len(dst)
	if n > available {
		n = available
	}
	for i := 0; i < n; i++ {
		dst[i] = rb.buf[int((r+uint64(i))%uint64(len(rb.buf)))]
	}
	rb.r.Store(r + uint64(n))
	return n
}
