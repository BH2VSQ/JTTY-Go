package radio

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RigctldConfig describes a connection to the Hamlib rigctld NET backend.
// rigctld uses the same command grammar as rigctl; see Hamlib's rigctld manual.
type RigctldConfig struct {
	Address    string
	Port       int
	Mode       string
	PassbandHz int
}

type RigctldRadio struct {
	mu      sync.Mutex
	conn    net.Conn
	cfg     RigctldConfig
	lastPTT bool
}

func NewRigctld(cfg RigctldConfig) *RigctldRadio {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 4532
	}
	if cfg.Mode == "" {
		cfg.Mode = "PKTUSB"
	}
	if cfg.PassbandHz <= 0 {
		cfg.PassbandHz = 3000
	}
	return &RigctldRadio{cfg: cfg}
}

func (r *RigctldRadio) Open(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil {
		return nil
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(r.cfg.Address, strconv.Itoa(r.cfg.Port)))
	if err != nil {
		return fmt.Errorf("connect rigctld: %w", err)
	}
	r.conn = conn
	return nil
}

func (r *RigctldRadio) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return nil
	}
	err := r.conn.Close()
	r.conn = nil
	return err
}

func (r *RigctldRadio) command(ctx context.Context, cmd string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return nil, fmt.Errorf("rigctld is not connected")
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = r.conn.SetDeadline(deadline)
	} else {
		_ = r.conn.SetDeadline(time.Now().Add(2 * time.Second))
	}
	if !strings.HasSuffix(cmd, "\n") {
		cmd += "\n"
	}
	if _, err := r.conn.Write([]byte(cmd)); err != nil {
		return nil, fmt.Errorf("rigctld write: %w", err)
	}
	s := bufio.NewScanner(r.conn)
	var lines []string
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		lines = append(lines, line)
		if strings.HasPrefix(line, "RPRT ") {
			break
		}
		// Simple protocol get-commands return their value line followed by RPRT.
		// Some servers may omit the trailer for compatibility, so one extra scan
		// timeout naturally ends the transaction.
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("rigctld read: %w", err)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "RPRT ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] != "0" {
				return lines, fmt.Errorf("rigctld error %s", line)
			}
		}
	}
	return lines, nil
}

func firstValue(lines []string) string {
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "RPRT ") {
			continue
		}
		return strings.Fields(line)[0]
	}
	return ""
}

var supportedLevelTokens = map[string]struct{}{
	"STRENGTH":            {},
	"ALC":                 {},
	"SWR":                 {},
	"RFPOWER_METER":       {},
	"RFPOWER_METER_WATTS": {},
}

func parseSupportedLevels(lines []string) map[string]bool {
	result := make(map[string]bool)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "RPRT ") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "Level:"))
		for _, token := range strings.Fields(line) {
			token = strings.ToUpper(strings.TrimSpace(token))
			if _, ok := supportedLevelTokens[token]; ok {
				result[token] = true
			}
		}
	}
	return result
}

// SupportedLevels queries the Hamlib get_level capability list.
func (r *RigctldRadio) SupportedLevels(ctx context.Context) (map[string]bool, error) {
	lines, err := r.command(ctx, "l ?")
	if err != nil {
		return nil, err
	}
	return parseSupportedLevels(lines), nil
}

func (r *RigctldRadio) Level(ctx context.Context, name string) (float64, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if _, ok := supportedLevelTokens[name]; !ok {
		return 0, fmt.Errorf("unsupported meter level %q", name)
	}
	lines, err := r.command(ctx, "l "+name)
	if err != nil {
		return 0, err
	}
	v := firstValue(lines)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s level %q", name, v)
	}
	return f, nil
}

func (r *RigctldRadio) Frequency(ctx context.Context) (int64, error) {
	lines, err := r.command(ctx, "f")
	if err != nil {
		return 0, err
	}
	v := firstValue(lines)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid frequency %q", v)
	}
	return int64(f + 0.5), nil
}

func (r *RigctldRadio) SetFrequency(ctx context.Context, hz int64) error {
	_, err := r.command(ctx, fmt.Sprintf("F %d", hz))
	return err
}

func (r *RigctldRadio) Mode(ctx context.Context) (string, error) {
	lines, err := r.command(ctx, "m")
	if err != nil {
		return "", err
	}
	return firstValue(lines), nil
}

func (r *RigctldRadio) SetMode(ctx context.Context, mode string) error {
	if mode == "" {
		mode = r.cfg.Mode
	}
	_, err := r.command(ctx, fmt.Sprintf("M %s %d", mode, r.cfg.PassbandHz))
	return err
}

func (r *RigctldRadio) PTT(ctx context.Context, on bool) error {
	v := 0
	if on {
		v = 1
	}
	_, err := r.command(ctx, fmt.Sprintf("T %d", v))
	if err == nil {
		r.mu.Lock()
		r.lastPTT = on
		r.mu.Unlock()
	}
	return err
}

func (r *RigctldRadio) Transmitting() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastPTT
}

func (r *RigctldRadio) SetSplit(ctx context.Context, txVFO string) error {
	if txVFO == "" {
		txVFO = "VFOB"
	}
	_, err := r.command(ctx, fmt.Sprintf("S 1 %s", txVFO))
	return err
}

func (r *RigctldRadio) SetSplitFrequency(ctx context.Context, hz int64) error {
	_, err := r.command(ctx, fmt.Sprintf("I %d", hz))
	return err
}

func (r *RigctldRadio) SplitFrequency(ctx context.Context) (int64, error) {
	lines, err := r.command(ctx, "i")
	if err != nil {
		return 0, err
	}
	v := firstValue(lines)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid split frequency %q", v)
	}
	return int64(f + 0.5), nil
}
