package jtty

import "testing"

func TestMessageAssemblerContinuationAndCompletion(t *testing.T) {
	a := NewMessageAssembler()
	period := float64(nFrameSymbols*nSS) / fs6k
	updates, accepted := a.Add(FrameDecode{FrequencyHz: 1500, XDT: 0, Decoded: "WB9XYZ", TrailingSep: true}, period)
	if !accepted || len(updates) != 1 || updates[0].Text != "WB9XYZ" {
		t.Fatalf("start=%#v accepted=%v", updates, accepted)
	}
	updates, accepted = a.Add(FrameDecode{FrequencyHz: 1500.7, XDT: period, Decoded: "599 123", TrailingSep: true, LastFrame: true}, period)
	if !accepted || len(updates) != 1 {
		t.Fatalf("continuation=%#v accepted=%v", updates, accepted)
	}
	if updates[0].Text != "WB9XYZ 599 123" || !updates[0].Complete {
		t.Fatalf("merged=%#v", updates[0])
	}
	if a.ActiveCount() != 0 {
		t.Fatalf("active=%d", a.ActiveCount())
	}
}

func TestMessageAssemblerRejectsRecentDuplicate(t *testing.T) {
	a := NewMessageAssembler()
	p := float64(nFrameSymbols*nSS) / fs6k
	_, ok := a.Add(FrameDecode{FrequencyHz: 1500, XDT: .1, Decoded: "CQ K1ABC CQ", LastFrame: true}, p)
	if !ok {
		t.Fatal("first not accepted")
	}
	_, ok = a.Add(FrameDecode{FrequencyHz: 1505, XDT: .12, Decoded: "CQ K1ABC CQ", LastFrame: true}, p)
	if ok {
		t.Fatal("recent duplicate accepted")
	}
}

func TestMessageAssemblerIDsRemainUniqueAcrossResetAndInstances(t *testing.T) {
	a := NewMessageAssembler()
	frames := []FrameDecode{{FrequencyHz: 1500, XDT: 1.0, Decoded: "A", LastFrame: true}}
	first, accepted := a.Add(frames[0], 0.47)
	if !accepted || len(first) != 1 || first[0].MessageID == 0 {
		t.Fatalf("first add = %#v accepted=%v", first, accepted)
	}
	firstID := first[0].MessageID

	a.Reset()
	second, accepted := a.Add(FrameDecode{FrequencyHz: 1500, XDT: 2.0, Decoded: "B", LastFrame: true}, 0.47)
	if !accepted || len(second) != 1 {
		t.Fatalf("second add = %#v accepted=%v", second, accepted)
	}
	if second[0].MessageID == firstID {
		t.Fatalf("reset reused message ID %d", firstID)
	}

	b := NewMessageAssembler()
	third, accepted := b.Add(FrameDecode{FrequencyHz: 1500, XDT: 3.0, Decoded: "C", LastFrame: true}, 0.47)
	if !accepted || len(third) != 1 {
		t.Fatalf("third add = %#v accepted=%v", third, accepted)
	}
	if third[0].MessageID == firstID || third[0].MessageID == second[0].MessageID {
		t.Fatalf("new assembler reused message ID: first=%d second=%d third=%d", firstID, second[0].MessageID, third[0].MessageID)
	}
}
