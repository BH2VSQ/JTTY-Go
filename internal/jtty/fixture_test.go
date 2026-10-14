package jtty

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/BH2VSQ/jtty-go/internal/decoder"
	"os"
	"strings"
	"testing"
)

func TestPinnedWSJTXFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/wsjtx-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Commit    string `json:"commit"`
		PCMSHA256 string `json:"pcmSha256"`
		Vectors   []struct {
			Message, Normalized string
			Bits                []string
			Tones               string
		} `json:"vectors"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Commit != WSJTXReference.Commit {
		t.Fatal("fixture/reference commit mismatch")
	}
	for _, v := range fixture.Vectors {
		frames, text, err := PackMessage(v.Message, ExchangeUnknown)
		if err != nil || text != v.Normalized || len(frames) != len(v.Bits) {
			t.Fatalf("%s: %v", v.Message, err)
		}
		var tones strings.Builder
		for i, f := range frames {
			if fmt.Sprintf("%034b", f.Bits) != v.Bits[i] {
				t.Fatalf("%s frame %d", v.Message, i)
			}
			encoded := EncodeFrame(payloadFromFrame(f))
			for _, tone := range encoded {
				fmt.Fprint(&tones, tone)
			}
		}
		if tones.String() != v.Tones {
			t.Fatalf("%s tones differ", v.Message)
		}
	}
	raw, err := os.ReadFile("testdata/wsjtx-cq.pcm")
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != fixture.PCMSHA256 {
		t.Fatal("reference PCM checksum mismatch")
	}
	samples := make([]float32, len(raw)/2)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(raw[2*i:])))
	}
	got, err := NewDecoder().Decode(context.Background(), decoder.Job{Samples: samples, FrequencyHz: 1500})
	if err != nil || len(got) != 1 || got[0].Message != "CQ K1ABC CQ" {
		t.Fatalf("reference PCM: %v %+v", err, got)
	}
}
