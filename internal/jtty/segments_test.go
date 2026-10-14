package jtty

import (
	"strings"
	"testing"
)

func TestContinuousSegmentsAndFillerAssembly(t *testing.T) {
	if _, text, err := PackSegment("  hello   world  ", ExchangeUnknown, false); err != nil || text != "HELLO WORLD" {
		t.Fatalf("normalization %q %v", text, err)
	}
	message := strings.Repeat("HELLO WORLD ", 20) + "GOODBYE"
	segments, canonical, err := PackTransmitLine(message, ExchangeUnknown)
	if err != nil || canonical != message || len(segments) < 3 {
		t.Fatalf("segments=%+v canonical=%q err=%v", segments, canonical, err)
	}
	a := NewMessageAssembler()
	index := 0
	var last MessageUpdate
	for _, segment := range segments {
		for _, frame := range segment.Frames {
			text, sep, eom, ok := UnpackMessage([]PackedFrame{frame})
			if !ok {
				t.Fatal("invalid frame")
			}
			updates, accepted := a.Add(FrameDecode{FrequencyHz: 1500, XDT: float64(index) * 1.888, SNRDB: -5, Decoded: text, TrailingSep: sep, LastFrame: eom}, 1.888)
			if !accepted {
				t.Fatal("frame rejected")
			}
			last = updates[0]
			index++
		}
	}
	if last.Text != message || !last.Complete || last.Terminal != "complete" {
		t.Fatalf("assembled %q state=%s", last.Text, last.Terminal)
	}
	if got := displayMessageText("HELLO <<<<< <<<<< WORLD"); got != "HELLO WORLD" {
		t.Fatal(got)
	}
	if got := displayMessageText("<<<<< <<<<<"); got != "" {
		t.Fatal(got)
	}
}

func TestAssemblyTerminalAndInitialSNR(t *testing.T) {
	a := NewMessageAssembler()
	a.Add(FrameDecode{FrequencyHz: 1500, XDT: 0, SNRDB: -10, Decoded: "HELLO", TrailingSep: true}, 1.888)
	updates, _ := a.Add(FrameDecode{FrequencyHz: 1500, XDT: 1.888, SNRDB: 5, Decoded: "WORLD"}, 1.888)
	if updates[0].SNRDB != -10 {
		t.Fatal("first-frame SNR changed")
	}
	expired := a.Prune(20, 1.888)
	if len(expired) != 1 || expired[0].Terminal != "expired" {
		t.Fatalf("%+v", expired)
	}
	a.Add(FrameDecode{FrequencyHz: 1500, XDT: 25, Decoded: "PARTIAL"}, 1.888)
	ended := a.EndReception()
	if len(ended) != 1 || ended[0].Terminal != "reception-ended" || a.ActiveCount() != 0 {
		t.Fatalf("%+v", ended)
	}
}
