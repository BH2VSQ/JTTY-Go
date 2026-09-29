package eventbus

import (
	"sync"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

type Handler func(payload any)

type Bus struct {
	mu       sync.RWMutex
	handlers map[model.EventName][]Handler
}

func New() *Bus {
	return &Bus{handlers: make(map[model.EventName][]Handler)}
}

func (b *Bus) Subscribe(event model.EventName, handler Handler) func() {
	b.mu.Lock()
	b.handlers[event] = append(b.handlers[event], handler)
	idx := len(b.handlers[event]) - 1
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		handlers := b.handlers[event]
		if idx >= 0 && idx < len(handlers) {
			handlers[idx] = nil
		}
	}
}

func (b *Bus) Publish(event model.EventName, payload any) {
	b.mu.RLock()
	snapshot := append([]Handler(nil), b.handlers[event]...)
	b.mu.RUnlock()

	for _, handler := range snapshot {
		if handler != nil {
			handler(payload)
		}
	}
}
