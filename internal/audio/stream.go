package audio

import "time"

type PCMFrame struct {
	Timestamp  time.Time
	Samples    []float32
	SampleRate int
	Channels   int
}

type FrameSink interface{ OnPCMFrame(PCMFrame) }
