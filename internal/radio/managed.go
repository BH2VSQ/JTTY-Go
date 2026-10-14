package radio

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ManagedRadio owns exactly one Hamlib process. Closing NET clients never uses it.
type ManagedRadio struct {
	*RigctldRadio
	cmd       *exec.Cmd
	done      chan struct{}
	closeOnce sync.Once
	log       tailBuffer
	closeErr  error
}

type tailBuffer struct {
	mu   sync.Mutex
	text string
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.text += string(p)
	if len(b.text) > 16384 {
		b.text = b.text[len(b.text)-16384:]
	}
	return len(p), nil
}
func (b *tailBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text }

// StartManaged binds a private loopback port. A child must remain alive through
// a real frequency transaction before it can become the active radio.
func StartManaged(ctx context.Context, executable string, args []string, cfg RigctldConfig) (*ManagedRadio, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		runArgs := append(append([]string{}, args...), "-T", "127.0.0.1", "-t", strconv.Itoa(port))
		cfg.Address = "127.0.0.1"
		cfg.Port = port
		m := &ManagedRadio{RigctldRadio: NewRigctld(cfg), done: make(chan struct{})}
		m.cmd = newHamlibCommand(executable, runArgs...)
		m.cmd.Dir = filepath.Dir(executable)
		m.cmd.Stdout = &m.log
		m.cmd.Stderr = &m.log
		if err = m.cmd.Start(); err != nil {
			return nil, fmt.Errorf("启动 Hamlib: %w", err)
		}
		go func() { _ = m.cmd.Wait(); close(m.done) }()
		readyCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		for readyCtx.Err() == nil {
			select {
			case <-m.done:
				last = fmt.Errorf("Hamlib 已退出: %s", m.log.String())
				goto failed
			default:
			}
			probe, c := context.WithTimeout(readyCtx, 200*time.Millisecond)
			err = m.Open(probe)
			if err == nil {
				_, err = m.Frequency(probe)
			}
			c()
			if err == nil {
				select {
				case <-m.done:
					last = fmt.Errorf("Hamlib 启动失败: %s", m.log.String())
					goto failed
				default:
				}
				cancel()
				return m, nil
			}
			last = err
			select {
			case <-readyCtx.Done():
			case <-time.After(40 * time.Millisecond):
			}
		}
		last = fmt.Errorf("Hamlib 就绪超时: %v; %s", last, m.log.String())
	failed:
		cancel()
		_ = m.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !strings.Contains(strings.ToLower(m.log.String()), "bind") {
			break
		}
	}
	return nil, last
}

func (m *ManagedRadio) Close() error {
	m.closeOnce.Do(func() {
		m.closeErr = m.RigctldRadio.Close()
		select {
		case <-m.done:
			return
		default:
		}
		_ = m.cmd.Process.Kill()
		<-m.done
	})
	return m.closeErr
}
func (m *ManagedRadio) Diagnostics() string { return m.log.String() }
func (m *ManagedRadio) Running() bool {
	select {
	case <-m.done:
		return false
	default:
		return true
	}
}
