package audio

import "context"

type OutputConfig struct {
	DeviceID   string
	SampleRate int
	Channels   int
	BufferMS   int
	Channel    string
	Level      int
}

type Output interface {
	Open(context.Context, OutputConfig) error
	Play(context.Context, []float32, int) error
	Stop() error
	Close() error
}
