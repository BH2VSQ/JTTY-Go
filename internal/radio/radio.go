package radio

import "context"

type Radio interface {
	Open(ctx context.Context) error
	Close() error
	Frequency(ctx context.Context) (int64, error)
	SetFrequency(ctx context.Context, hz int64) error
	Mode(ctx context.Context) (string, error)
	SetMode(ctx context.Context, mode string) error
	PTT(ctx context.Context, on bool) error
	Transmitting() bool
}

// MeterReader exposes optional Hamlib get-level capabilities.
// It stays separate from Radio so existing backends remain source-compatible.
type MeterReader interface {
	SupportedLevels(ctx context.Context) (map[string]bool, error)
	Level(ctx context.Context, name string) (float64, error)
}
