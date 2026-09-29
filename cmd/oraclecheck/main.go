package main

import (
	"context"
	"fmt"
	"github.com/BH2VSQ/jtty-go/internal/decoder"
	"github.com/BH2VSQ/jtty-go/internal/jtty"
	"time"
)

func main() {
	frames, norm, err := jtty.PackMessage("599 123", jtty.ExchangeUnknown)
	fmt.Printf("frames=%d norm=%q err=%v\n", len(frames), norm, err)
	if err != nil || len(frames) == 0 {
		return
	}
	fmt.Printf("bits=%09X\n", frames[0].Bits)
	var p [jtty.PayloadBits]int
	v := frames[0].Bits
	for i := 0; i < jtty.PayloadBits; i++ {
		if v&(uint64(1)<<uint(jtty.PayloadBits-1-i)) != 0 {
			p[i] = 1
		}
	}
	tones := jtty.EncodeFrame(p)
	wave := jtty.GenerateJTTYWaveform(tones[:], 384, 2, 12000, 1500)
	samples := make([]float32, len(wave))
	for i, x := range wave {
		samples[i] = float32(x * 14000)
	}
	d := jtty.NewDecoder()
	msgs, err := d.Decode(context.Background(), decoder.Job{CandidateID: 1, Samples: samples, FrequencyHz: 1500, Timestamp: time.Unix(0, 0).UTC()})
	fmt.Printf("decode err=%v msgs=%#v\n", err, msgs)
}
