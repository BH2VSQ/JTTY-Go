package jtty

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/BH2VSQ/jtty-go/internal/decoder"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// JTTY_ORACLE points to the harness linked against the pinned, unmodified
// upstream Fortran sources. These tests never substitute Go-generated input
// for the independent encode direction.
func TestIndependentWSJTXOracle(t *testing.T) {
	oracle := os.Getenv("JTTY_ORACLE")
	if oracle == "" {
		t.Skip("set JTTY_ORACLE to the independently built WSJT-X harness")
	}
	for _, message := range []string{"CQ K1ABC CQ", "599 123", "599 CA", "599 05 NWT", "1D EMA", "599 156 1749", "AGN?", "FN42", "HELLO", "K1ABC AGN?"} {
		t.Run(message, func(t *testing.T) {
			pcmPath := filepath.Join(t.TempDir(), "upstream.pcm")
			output, err := exec.Command(oracle, "encode", message, pcmPath).CombinedOutput()
			if err != nil {
				t.Fatalf("Fortran encode: %v %s", err, output)
			}
			lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(output), "\r", "")), "\n")
			frames, normalized, err := PackMessage(message, ExchangeUnknown)
			if err != nil {
				t.Fatal(err)
			}
			count, _ := strconv.Atoi(strings.TrimSpace(lines[1]))
			if count != len(frames) || lines[0] != normalized {
				t.Fatalf("Fortran count/text: %s", output)
			}
			var goTones strings.Builder
			for i, f := range frames {
				bits, err := strconv.ParseUint(strings.TrimSpace(lines[2+i]), 2, 64)
				if err != nil || bits != f.Bits {
					t.Fatalf("frame %d Go=%034b Fortran=%s", i, f.Bits, lines[2+i])
				}
				tones := EncodeFrame(payloadFromFrame(f))
				for _, tone := range tones {
					fmt.Fprint(&goTones, tone)
				}
			}
			if goTones.String() != lines[2+count] {
				t.Fatal("Go/Fortran channel tones differ")
			}
			raw, err := os.ReadFile(pcmPath)
			if err != nil {
				t.Fatal(err)
			}
			samples := make([]float32, len(raw)/2)
			for i := range samples {
				samples[i] = float32(int16(binary.LittleEndian.Uint16(raw[2*i:])))
			}
			d := NewDecoder()
			for i, f := range frames {
				start := i * 22656
				chunk := samples[start:min(start+28320, len(samples))]
				got, err := d.Decode(context.Background(), decoder.Job{Samples: chunk, FrequencyHz: 1500, Timestamp: time.Unix(0, 0).UTC()})
				text, _, _, _ := UnpackMessage([]PackedFrame{f})
				if err != nil || len(got) == 0 || got[0].Message != displayJTTYText(text) {
					t.Fatalf("Fortran -> Go frame %d: %v %+v want %q", i, err, got, text)
				}
				// Independent reverse direction, including payloads from structured exchanges.
				tones := EncodeFrame(payloadFromFrame(f))
				wave := GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
				goPCM := make([]byte, (len(wave)+7200)*2)
				for j, x := range wave {
					binary.LittleEndian.PutUint16(goPCM[2*(j+2400):], uint16(int16(x*14000)))
				}
				goPath := filepath.Join(t.TempDir(), "go.pcm")
				if err := os.WriteFile(goPath, goPCM, 0600); err != nil {
					t.Fatal(err)
				}
				decoded, err := exec.Command(oracle, "decode", "", goPath).CombinedOutput()
				if err != nil || displayJTTYText(strings.TrimSpace(string(decoded))) != text {
					t.Fatalf("Go -> Fortran frame %d: %v %q want %q", i, err, decoded, text)
				}
			}
		})
	}
}

func TestIndependentWeakSignalSNRAndOverlap(t *testing.T) {
	oracle := os.Getenv("JTTY_ORACLE")
	if oracle == "" {
		t.Skip("set JTTY_ORACLE to the independently built WSJT-X harness")
	}
	for _, scenario := range []string{"weak", "overlap"} {
		t.Run(scenario, func(t *testing.T) {
			messages := []string{"CQ K1ABC CQ"}
			frequencies := []int{1500}
			if scenario == "overlap" {
				messages = append(messages, "CQ W9XYZ CQ")
				frequencies = []int{1450, 1550}
			}
			samples := make([]float32, 33000)
			for i, message := range messages {
				path := filepath.Join(t.TempDir(), "reference.pcm")
				if output, err := exec.Command(oracle, "encode", message, path, strconv.Itoa(frequencies[i])).CombinedOutput(); err != nil {
					t.Fatalf("%v %s", err, output)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				for j := 0; j < len(raw)/2; j++ {
					samples[j] += float32(int16(binary.LittleEndian.Uint16(raw[2*j:])))
				}
			}
			if scenario == "weak" {
				// Deterministic AWGN at approximately -8 dB in 2500 Hz.
				rng := rand.New(rand.NewSource(73))
				for i := range samples {
					samples[i] = samples[i]*0.12 + float32(rng.NormFloat64()*4630)
				}
			}
			raw := make([]byte, len(samples)*2)
			for i, sample := range samples {
				sample = float32(math.Max(-32767, math.Min(32767, float64(sample))))
				samples[i] = sample
				binary.LittleEndian.PutUint16(raw[2*i:], uint16(int16(sample)))
			}
			path := filepath.Join(t.TempDir(), "scenario.pcm")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			upstream, err := exec.Command(oracle, "decode-many", "", path).CombinedOutput()
			if err != nil {
				t.Fatalf("Fortran failed %v %s", err, upstream)
			}
			d := NewDecoder()
			got, err := d.DecodeWidebandManyDetailed(context.Background(), decoder.Job{Samples: samples, FrequencyHz: 1500}, 4, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range messages {
				if !strings.Contains(string(upstream), message) {
					t.Fatalf("Fortran omitted %q: %s", message, upstream)
				}
				found := false
				for _, hit := range got {
					if hit.Message.Message == message {
						found = true
						if scenario == "weak" {
							for _, line := range strings.Split(strings.TrimSpace(string(upstream)), "\n") {
								parts := strings.SplitN(line, "|", 2)
								if len(parts) == 2 && strings.TrimSpace(parts[1]) == message {
									snr, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
									if abs(float64(snr-hit.Message.SNR)) > 2 {
										t.Fatalf("SNR Go=%d Fortran=%d", hit.Message.SNR, snr)
									}
								}
							}
						}
					}
				}
				if !found {
					t.Fatalf("Go omitted %q: %+v", message, got)
				}
			}
		})
	}
}
