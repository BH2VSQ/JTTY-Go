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
	mu        sync.Mutex
	conn      net.Conn
	reader    *bufio.Reader
	pttConn   net.Conn
	pttReader *bufio.Reader
	cfg       RigctldConfig
	lastPTT   bool
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
	r.reader = bufio.NewReader(conn)
	return nil
}

func (r *RigctldRadio) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	if r.pttConn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, firstErr = rigctldPTTCommand(ctx, r.pttConn, r.pttReader, 0)
		cancel()
		if err := r.pttConn.Close(); firstErr == nil && err != nil {
			firstErr = fmt.Errorf("close rigctld PTT session: %w", err)
		}
		r.pttConn = nil
		r.pttReader = nil
		r.lastPTT = false
	}
	if r.conn != nil {
		if err := r.conn.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
		r.conn = nil
		r.reader = nil
	}
	return firstErr
}

func (r *RigctldRadio) command(ctx context.Context, cmd string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return nil, fmt.Errorf("rigctld is not connected")
	}
	if r.reader == nil {
		r.reader = bufio.NewReader(r.conn)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(2 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := r.conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("rigctld set deadline: %w", err)
	}

	command := strings.TrimSpace(cmd)
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty rigctld command")
	}
	// Default rigctld protocol returns getter values without an RPRT success line.
	// Do not wait for RPRT after a successful GET, or the read will time out.
	verb := fields[0]
	valueLines := 0
	switch verb {
	case "f", "t", "i", "l":
		valueLines = 1
	case "m":
		valueLines = 2 // Mode and passband, each on its own line.
	}

	if _, err := r.conn.Write([]byte(command + "\n")); err != nil {
		return nil, fmt.Errorf("rigctld write: %w", err)
	}
	readLine := func() (string, error) {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("rigctld read: %w", err)
		}
		return strings.TrimSpace(line), nil
	}
	parseRPRT := func(line string) error {
		if !strings.HasPrefix(line, "RPRT ") {
			return nil
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return fmt.Errorf("invalid rigctld response %q", line)
		}
		code, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid rigctld response %q: %w", line, err)
		}
		if code != 0 {
			return fmt.Errorf("rigctld error %s", line)
		}
		return nil
	}

	first, err := readLine()
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(first, "RPRT ") {
		if err := parseRPRT(first); err != nil {
			return []string{first}, err
		}
		if valueLines > 0 {
			return []string{first}, fmt.Errorf("rigctld returned success status without a value for %q", command)
		}
		return []string{first}, nil
	}

	if valueLines == 0 {
		return []string{first}, fmt.Errorf("rigctld returned %q for set command %q; expected RPRT", first, command)
	}
	lines := []string{first}
	for len(lines) < valueLines {
		line, readErr := readLine()
		if readErr != nil {
			return lines, readErr
		}
		if strings.HasPrefix(line, "RPRT ") {
			if err := parseRPRT(line); err != nil {
				return lines, err
			}
			// Some rig backends omit the optional passband value. The success
			// trailer is consumed here so it cannot poison the next transaction.
			break
		}
		lines = append(lines, line)
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
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return fmt.Errorf("rigctld is not connected")
	}

	if on {
		if r.pttConn != nil {
			// An active PTT session must not be recreated or have its safety
			// timeout implicitly extended by a duplicate ON command.
			return nil
		}
		dialer := net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(r.cfg.Address, strconv.Itoa(r.cfg.Port)))
		if err != nil {
			return fmt.Errorf("connect rigctld PTT session: %w", err)
		}
		reader := bufio.NewReader(conn)
		response, err := rigctldPTTCommand(ctx, conn, reader, 1)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("rigctld PTT-on failed (response %q): %w", response, err)
		}
		r.pttConn = conn
		r.pttReader = reader
		r.lastPTT = true
		return nil
	}

	if r.pttConn != nil {
		// PTT-OFF uses exactly the same TCP session as PTT-ON. A number of
		// rigctld backends release keying when that session is closed.
		response, err := rigctldPTTCommand(ctx, r.pttConn, r.pttReader, 0)
		closeErr := r.pttConn.Close()
		r.pttConn = nil
		r.pttReader = nil
		r.lastPTT = false
		if err != nil {
			return fmt.Errorf("rigctld PTT-off failed (response %q): %w", response, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close rigctld PTT session: %w", closeErr)
		}
		return nil
	}

	// A safe OFF command is still sent if the radio was not keyed through this
	// instance, for example during initial connection or after recovery.
	r.mu.Unlock()
	_, err := r.command(ctx, "T 0")
	r.mu.Lock()
	if err == nil {
		r.lastPTT = false
	}
	return err
}

// rigctldPTTCommand is the small, line-oriented PTTSession protocol used by
// SATAprs-Go. The connection is kept open by the caller after T 1 and reused
// for T 0. Other radio status queries must not use this PTT socket.
func rigctldPTTCommand(ctx context.Context, conn net.Conn, reader *bufio.Reader, value int) (string, error) {
	if value < 0 || value > 3 {
		return "", fmt.Errorf("invalid rigctld PTT value %d", value)
	}
	if conn == nil || reader == nil {
		return "", fmt.Errorf("rigctld PTT session is not open")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	deadline := time.Now().Add(2 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", fmt.Errorf("set rigctld PTT deadline: %w", err)
	}
	if _, err := conn.Write([]byte(fmt.Sprintf("T %d\n", value))); err != nil {
		return "", fmt.Errorf("write rigctld PTT command: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read rigctld PTT response: %w", err)
	}
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "RPRT ") {
		code, parseErr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "RPRT ")))
		if parseErr != nil {
			return line, fmt.Errorf("invalid rigctld PTT response %q", line)
		}
		if code != 0 {
			return line, fmt.Errorf("rigctld rejected PTT command with %s", line)
		}
	}
	return line, nil
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
