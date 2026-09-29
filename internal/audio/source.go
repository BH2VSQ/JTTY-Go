package audio

import "context"

type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsInput     bool   `json:"isInput"`
	IsOutput    bool   `json:"isOutput"`
	SampleRates []int  `json:"sampleRates,omitempty"`
}

type CaptureConfig struct {
	DeviceID    string
	SampleRate  int
	Channels    int
	BufferMS    int
	ChannelMode string
}

type Source interface {
	Open(ctx context.Context, cfg CaptureConfig, sink FrameSink) error
	Start(ctx context.Context) error
	Stop() error
	Close() error
}

type Manager interface {
	Inputs() ([]Device, error)
	Outputs() ([]Device, error)
	NewCapture() Source
	NewOutput() Output
}

type PCMFormat struct {
	SampleRate int `json:"sampleRate"`
	Channels   int `json:"channels"`
}
