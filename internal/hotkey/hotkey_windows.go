//go:build windows

package hotkey

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmHotKey    = 0x0312
	pmRemove    = 0x0001
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

type pointMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
	private uint32
}

type command struct {
	register   *Binding
	unregister string
	response   chan error
}

type windowsManager struct {
	commands  chan command
	done      chan struct{}
	wg        sync.WaitGroup
	handlerMu sync.RWMutex
	handler   Handler
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procPeekMessageW     = user32.NewProc("PeekMessageW")
)

func NewManager() Manager {
	m := &windowsManager{commands: make(chan command, 64), done: make(chan struct{})}
	m.wg.Add(1)
	go m.loop()
	return m
}

func (m *windowsManager) SetHandler(h Handler) {
	m.handlerMu.Lock()
	m.handler = h
	m.handlerMu.Unlock()
}

func (m *windowsManager) Register(ctx context.Context, binding Binding) error {
	if !binding.Enabled {
		return nil
	}
	return m.send(ctx, command{register: &binding, response: make(chan error, 1)})
}

func (m *windowsManager) Unregister(id string) error {
	return m.send(context.Background(), command{unregister: id, response: make(chan error, 1)})
}

func (m *windowsManager) Close() error {
	select {
	case <-m.done:
		return nil
	default:
	}
	close(m.done)
	m.wg.Wait()
	return nil
}

func (m *windowsManager) send(ctx context.Context, cmd command) error {
	select {
	case <-m.done:
		return fmt.Errorf("hotkey manager is closed")
	case <-ctx.Done():
		return ctx.Err()
	case m.commands <- cmd:
	}
	select {
	case err := <-cmd.response:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *windowsManager) loop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer m.wg.Done()
	ids := make(map[string]int)
	nextID := 0x5100
	for {
		select {
		case <-m.done:
			for _, id := range ids {
				_, _, _ = procUnregisterHotKey.Call(0, uintptr(id))
			}
			return
		case cmd := <-m.commands:
			if cmd.register != nil {
				b := *cmd.register
				if old, ok := ids[b.ID]; ok {
					_, _, _ = procUnregisterHotKey.Call(0, uintptr(old))
					delete(ids, b.ID)
				}
				mods, vk, err := parseShortcut(b.Shortcut)
				if err == nil {
					id := nextID
					nextID++
					r1, _, e := procRegisterHotKey.Call(0, uintptr(id), uintptr(mods), uintptr(vk))
					if r1 != 0 {
						ids[b.ID] = id
					} else {
						err = e
						if err == nil {
							err = fmt.Errorf("RegisterHotKey failed for %q", b.Shortcut)
						}
					}
				}
				cmd.response <- err
			} else {
				if id, ok := ids[cmd.unregister]; ok {
					_, _, _ = procUnregisterHotKey.Call(0, uintptr(id))
					delete(ids, cmd.unregister)
				}
				cmd.response <- nil
			}
		default:
			var msg pointMsg
			for {
				r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, pmRemove)
				if r == 0 {
					break
				}
				if msg.message == wmHotKey {
					id := int(msg.wParam)
					for bindingID, registeredID := range ids {
						if registeredID == id {
							m.handlerMu.RLock()
							h := m.handler
							m.handlerMu.RUnlock()
							if h != nil {
								h(Binding{ID: bindingID, Name: bindingID, Shortcut: "", Global: true, Enabled: true})
							}
							break
						}
					}
				}
			}
			time.Sleep(4 * time.Millisecond)
		}
	}
}

func parseShortcut(s string) (uint32, uint16, error) {
	parts := strings.FieldsFunc(strings.ToUpper(strings.TrimSpace(s)), func(r rune) bool { return r == '+' || r == ' ' })
	if len(parts) == 0 {
		return 0, 0, fmt.Errorf("empty shortcut")
	}
	var mods uint32
	key := ""
	for _, p := range parts {
		switch p {
		case "CTRL", "CONTROL":
			mods |= modControl
		case "ALT":
			mods |= modAlt
		case "SHIFT":
			mods |= modShift
		case "WIN", "WINDOWS":
			mods |= modWin
		default:
			key = p
		}
	}
	mods |= modNoRepeat
	if len(key) == 1 {
		ch := key[0]
		switch {
		case ch >= 'A' && ch <= 'Z':
			return mods, uint16(ch), nil
		case ch >= '0' && ch <= '9':
			return mods, uint16(ch), nil
		}
	}
	if strings.HasPrefix(key, "F") {
		n, err := strconv.Atoi(strings.TrimPrefix(key, "F"))
		if err == nil && n >= 1 && n <= 24 {
			return mods, uint16(0x70 + n - 1), nil
		}
	}
	special := map[string]uint16{"SPACE": 0x20, "ENTER": 0x0D, "ESC": 0x1B, "ESCAPE": 0x1B, "TAB": 0x09, "LEFT": 0x25, "UP": 0x26, "RIGHT": 0x27, "DOWN": 0x28, "INSERT": 0x2D, "DELETE": 0x2E, "HOME": 0x24, "END": 0x23, "PAGEUP": 0x21, "PAGEDOWN": 0x22}
	if vk, ok := special[key]; ok {
		return mods, vk, nil
	}
	return 0, 0, fmt.Errorf("unsupported shortcut %q", s)
}
