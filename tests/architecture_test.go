package tests

import (
	"testing"

	"github.com/BH2VSQ/jtty-go/internal/audio"
	"github.com/BH2VSQ/jtty-go/internal/jtty"
	"github.com/BH2VSQ/jtty-go/internal/macro"
)

func TestRingBufferOrder(t *testing.T) {
	rb := audio.NewRingBuffer(8)
	in := []float32{1, 2, 3, 4}
	if n := rb.Write(in); n != len(in) {
		t.Fatalf("write=%d", n)
	}
	out := make([]float32, 4)
	if n := rb.Read(out); n != 4 {
		t.Fatalf("read=%d", n)
	}
	for i, v := range out {
		if v != in[i] {
			t.Fatalf("index %d got %v", i, v)
		}
	}
}
func TestRingBufferDropsOldAudio(t *testing.T) {
	rb := audio.NewRingBuffer(4)
	rb.Write([]float32{1, 2, 3, 4, 5, 6})
	out := make([]float32, 4)
	rb.Read(out)
	want := []float32{3, 4, 5, 6}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("%d got %v want %v", i, out[i], want[i])
		}
	}
	if rb.Dropped() != 2 {
		t.Fatalf("dropped=%d", rb.Dropped())
	}
}
func TestMacroRender(t *testing.T) {
	got := macro.Render("%H 599 %E", macro.Context{DXCall: "JA1ABC", Exchange: "102"})
	if got != "JA1ABC 599 102" {
		t.Fatalf("got %q", got)
	}
}
func TestCallsignExtraction(t *testing.T) {
	got := jtty.ExtractCallsigns("CQ JA1ABC PM95")
	if len(got) != 1 || got[0].Value != "JA1ABC" {
		t.Fatalf("unexpected %+v", got)
	}
}
