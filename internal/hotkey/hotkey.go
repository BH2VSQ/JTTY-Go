package hotkey

import "context"

type Binding struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Shortcut string `json:"shortcut"`
	Global   bool   `json:"global"`
	Enabled  bool   `json:"enabled"`
}

type Handler func(Binding)

type Manager interface {
	SetHandler(handler Handler)
	Register(ctx context.Context, binding Binding) error
	Unregister(id string) error
	Close() error
}
