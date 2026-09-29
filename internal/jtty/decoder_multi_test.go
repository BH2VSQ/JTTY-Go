package jtty

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/decoder"
)

func payloadFromFrame(f PackedFrame) [PayloadBits]int {
	var p [PayloadBits]int
	for i := 0; i < PayloadBits; i++ {
		if f.Bits&(uint64(1)<<uint(PayloadBits-1-i)) != 0 {
			p[i] = 1
		}
	}
	return p
}

func TestDecodeGeneratedJTTY(t *testing.T) {
	frames, _, err := PackMessage("CQ K1ABC CQ", ExchangeUnknown)
	if err != nil || len(frames) != 1 {
		t.Fatalf("pack: frames=%d err=%v", len(frames), err)
	}
	p := payloadFromFrame(frames[0])
	tones := EncodeFrame(p)
	wave := GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
	samples := make([]float32, len(wave))
	for i, x := range wave {
		samples[i] = x * 12000
	}
	d := NewDecoder()
	got, err := d.Decode(context.Background(), decoder.Job{Samples: samples, FrequencyHz: 1500, Timestamp: time.Unix(0, 0).UTC()})
	if err != nil || len(got) != 1 {
		t.Fatalf("decode: err=%v got=%#v", err, got)
	}
	if got[0].Message != "CQ K1ABC CQ" {
		t.Fatalf("message=%q", got[0].Message)
	}
	if !got[0].JTTYLastFrame {
		t.Fatal("decoded final frame did not carry EOM state")
	}
}

func TestDecodeWidebandManyGeneratedJTTY(t *testing.T) {
	f1, _, err := PackMessage("CQ K1ABC CQ", ExchangeUnknown)
	if err != nil || len(f1) != 1 {
		t.Fatal(err)
	}
	f2, _, err := PackMessage("CQ W9XYZ CQ", ExchangeUnknown)
	if err != nil || len(f2) != 1 {
		t.Fatal(err)
	}
	t1 := EncodeFrame(payloadFromFrame(f1[0]))
	t2 := EncodeFrame(payloadFromFrame(f2[0]))
	w1 := GenerateJTTYWaveform(t1[:], 384, 2, 12000, 700)
	w2 := GenerateJTTYWaveform(t2[:], 384, 2, 12000, 2300)
	if len(w1) != len(w2) {
		t.Fatal("length mismatch")
	}
	samples := make([]float32, len(w1))
	for i := range samples {
		samples[i] = (w1[i] + w2[i]) * 7000
	}
	d := NewDecoder()
	start := time.Now()
	got, err := d.DecodeWidebandManyDetailed(context.Background(), decoder.Job{Samples: samples, FrequencyHz: 1500}, 4, true, nil)
	t.Logf("duration=%v got=%#v", time.Since(start), got)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range got {
		seen[m.Message.Message] = true
	}
	if !seen["CQ K1ABC CQ"] || !seen["CQ W9XYZ CQ"] {
		t.Fatalf("wideband missing: seen=%v got=%#v", seen, got)
	}
}

func TestDecodeManyGeneratedJTTY(t *testing.T) {
	f1, _, err := PackMessage("CQ K1ABC CQ", ExchangeUnknown)
	if err != nil || len(f1) != 1 {
		t.Fatal(err)
	}
	f2, _, err := PackMessage("CQ W9XYZ CQ", ExchangeUnknown)
	if err != nil || len(f2) != 1 {
		t.Fatal(err)
	}
	t1 := EncodeFrame(payloadFromFrame(f1[0]))
	t2 := EncodeFrame(payloadFromFrame(f2[0]))
	w1 := GenerateJTTYWaveform(t1[:], 384, 2, 12000, 1450)
	w2 := GenerateJTTYWaveform(t2[:], 384, 2, 12000, 1550)
	if len(w1) != len(w2) {
		t.Fatal("length mismatch")
	}
	samples := make([]float32, len(w1))
	for i := range samples {
		samples[i] = (w1[i] + w2[i]) * 7000
	}
	d := NewDecoder()
	start := time.Now()
	got, err := d.DecodeMany(context.Background(), decoder.Job{Samples: samples, FrequencyHz: 1500}, 4, true)
	dur := time.Since(start)
	t.Logf("duration=%v got=%#v", dur, got)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range got {
		seen[m.Message] = true
	}
	if !seen["CQ K1ABC CQ"] || !seen["CQ W9XYZ CQ"] {
		t.Fatalf("missing: seen=%v got=%#v", seen, got)
	}
	_ = math.Pi
}

func TestEOMPropagatesToDecodeMessage(t *testing.T) {
	job := decoder.Job{Timestamp: time.Date(2026, 9, 29, 3, 4, 5, 0, time.UTC)}
	final := decomposeDecodeMessage("CQ TEST CQ", job, 1500, 0.25, 8.0, true)
	if !final.JTTYLastFrame {
		t.Fatal("final-frame EOM was not propagated to DecodeMessage")
	}
	partial := decomposeDecodeMessage("599 001", job, 1500, 0.25, 8.0, false)
	if partial.JTTYLastFrame {
		t.Fatal("non-final frame was incorrectly marked as final")
	}
}

func TestGenerateJTTYWaveformIntoMatchesAllocated(t *testing.T) {
	frames, _, err := PackMessage("CQ BH2VSQ PN11", ExchangeUnknown)
	if err != nil || len(frames) == 0 {
		t.Fatalf("pack message: %v", err)
	}
	bits := payloadFromFrame(frames[0])
	tones := EncodeFrame(bits)
	want := GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
	got := make([]float32, len(want))
	dphi := make([]float64, (len(tones)+2)*384)
	pulse := make([]float64, 3*384)
	GenerateJTTYWaveformInto(got, dphi, pulse, tones[:], 384, 2, 12000, 1500)
	if len(want) != len(got) {
		t.Fatalf("length mismatch: want %d got %d", len(want), len(got))
	}
	for i := range want {
		if diff := math.Abs(float64(want[i] - got[i])); diff > 1e-7 {
			t.Fatalf("sample %d mismatch: want %.9f got %.9f", i, want[i], got[i])
		}
	}
}
