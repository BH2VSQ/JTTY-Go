//go:build !windows

package hotkey

import (
	"context"
	"fmt"
)

type stubManager struct{ handler Handler }

func NewManager() Manager                   { return &stubManager{} }
func (m *stubManager) SetHandler(h Handler) { m.handler = h }
func (m *stubManager) Register(ctx context.Context, binding Binding) error {
	return fmt.Errorf("global hotkeys are available only on Windows")
}
func (m *stubManager) Unregister(id string) error { return nil }
func (m *stubManager) Close() error               { return nil }
