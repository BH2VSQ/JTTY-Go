package logbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DecodeLogSingle/Year/Month control how the ALL decode log is partitioned.
const (
	DecodeLogSingle = "single"
	DecodeLogYear   = "year"
	DecodeLogMonth  = "month"
)

type DecodeLogger struct {
	mu sync.Mutex
}

func NewDecodeLogger() *DecodeLogger { return &DecodeLogger{} }

func (l *DecodeLogger) Append(dir, mode string, t time.Time, dialFrequencyHz int64, snr int, dt float64, df int, message string) error {
	if l == nil {
		return fmt.Errorf("nil decode logger")
	}
	if dir == "" {
		return fmt.Errorf("empty data directory")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	} else {
		t = t.UTC()
	}
	path := DecodeLogPath(dir, mode, t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	line := FormatDecodeLogLine(t, dialFrequencyHz, snr, dt, df, message)
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(line)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func (l *DecodeLogger) AppendTX(dir, mode string, t time.Time, dialFrequencyHz int64, txFrequencyHz int, message string) error {
	if l == nil {
		return fmt.Errorf("nil decode logger")
	}
	if dir == "" {
		return fmt.Errorf("empty data directory")
	}
	if t.IsZero() {
		t = time.Now().UTC()
	} else {
		t = t.UTC()
	}
	path := DecodeLogPath(dir, mode, t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	line := FormatTXLogLine(t, dialFrequencyHz, txFrequencyHz, message)
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(line)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func FormatTXLogLine(t time.Time, dialFrequencyHz int64, txFrequencyHz int, message string) string {
	t = t.UTC()
	freqMHz := float64(dialFrequencyHz) / 1e6
	cleanMessage := strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(strings.TrimSpace(message))), " ")
	return fmt.Sprintf("%s    %.3f Tx JTTY     0  0.0 %d %s\n", t.Format("060102_150405"), freqMHz, txFrequencyHz, cleanMessage)
}

func DecodeLogPath(dir, mode string, t time.Time) string {
	switch mode {
	case DecodeLogYear:
		return filepath.Join(dir, fmt.Sprintf("ALL-%04d.txt", t.UTC().Year()))
	case DecodeLogMonth:
		return filepath.Join(dir, fmt.Sprintf("ALL-%04d-%02d.txt", t.UTC().Year(), int(t.UTC().Month())))
	default:
		return filepath.Join(dir, "ALL.txt")
	}
}

func FormatDecodeLogLine(t time.Time, dialFrequencyHz int64, snr int, dt float64, df int, message string) string {
	t = t.UTC()
	freqMHz := float64(dialFrequencyHz) / 1e6
	message = strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(strings.TrimSpace(message))), " ")
	return fmt.Sprintf("%s    %.3f Rx JTTY     %d  %.1f %d %s\n", t.Format("060102_150405"), freqMHz, snr, dt, df, message)
}
