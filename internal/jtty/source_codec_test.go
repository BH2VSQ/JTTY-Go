package jtty

import "testing"

func TestWSJTXSourceEncodingVectors(t *testing.T) {
	tests := []struct {
		name string
		atom SourceAtom
		eom  bool
		hex  uint64
		text string
		last bool
	}{
		{"cq", SourceAtom{Kind: atomCall, Subtype: callCQ, Text: "K1ABC"}, true, 0x026F78D41, "CQ K1ABC CQ", true},
		{"serial123", SourceAtom{Kind: atomExchNum, Role: roleFull, Subtype: numSerial, Value: 123}, false, 0x20007B008, "599 123", false},
		{"serial123_eom", SourceAtom{Kind: atomExchNum, Role: roleFull, Subtype: numSerial, Value: 123}, true, 0x20007B009, "599 123", true},
		{"stateCA", SourceAtom{Kind: atomExchLoc, Role: roleFull, Subtype: locStateProvince, Text: "CA"}, true, 0x2001BA019, "599 CA", true},
		{"zoneLoc3", SourceAtom{Kind: atomExchPair, Subtype: pairZoneLoc3, Value: 5, Text: "NWT"}, true, 0x00B790D29, "599 05 NWT", true},
		{"classSection", SourceAtom{Kind: atomExchPair, Subtype: pairClassSection, Value: 1, Value2: sectionIndex("EMA"), Text: "D"}, true, 0x082C58029, "1D EMA", true},
		{"numTime", SourceAtom{Kind: atomExchNumTime, Role: roleFull, Value: 156, Value2: 17*60 + 49}, true, 0x204E42D39, "599 156 1749", true},
		{"control", SourceAtom{Kind: atomControl, Subtype: controlAGN}, true, 0x000000049, "AGN?", true},
		{"grid", SourceAtom{Kind: atomGrid4, Role: roleFieldOnly, Text: "FN42"}, true, 0x04A198049, "FN42", true},
		{"text5", SourceAtom{Kind: atomText5, Text: "HELLO"}, true, 0x11395558D, "HELLO", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			frame, ok := packAtom(tc.atom, tc.eom)
			if !ok {
				t.Fatal("packAtom rejected reference vector")
			}
			if frame.Bits != tc.hex {
				t.Fatalf("frame=%09X want %09X", frame.Bits, tc.hex)
			}
			atom, eom, valid := unpackAtom(frame)
			if !valid || eom != tc.eom {
				t.Fatalf("unpack valid=%v eom=%v", valid, eom)
			}
			got, valid := renderAtom(atom)
			if !valid || got != tc.text {
				t.Fatalf("render=%q want %q", got, tc.text)
			}
		})
	}
}

func TestPackMessageWSJTXFrameCounts(t *testing.T) {
	tests := []struct {
		message   string
		profile   ExchangeProfile
		frames    int
		canonical string
	}{
		{"CQ K1ABC CQ", ExchangeUnknown, 1, "CQ K1ABC CQ"},
		{"K1A", ExchangeUnknown, 1, "K1A"},
		{"WB9XYZ", ExchangeUnknown, 1, "WB9XYZ"},
		{"TU WB9XYZ CQ", ExchangeUnknown, 1, "TU WB9XYZ CQ"},
		{"WB9XYZ TU", ExchangeUnknown, 1, "WB9XYZ TU"},
		{"WB9XYZ AGN?", ExchangeUnknown, 1, "WB9XYZ AGN?"},
		{"TU NOW WB9XYZ", ExchangeUnknown, 1, "TU NOW WB9XYZ"},
		{"WB9XYZ 599 123", ExchangeUnknown, 2, "WB9XYZ 599 123"},
		{"599 MA", ExchangeUnknown, 1, "599 MA"},
		{"599 FN42", ExchangeUnknown, 1, "599 FN42"},
		{"FN42", ExchangeUnknown, 1, "FN42"},
		{"1D EMA", ExchangeUnknown, 1, "1D EMA"},
		{"32F EMA", ExchangeUnknown, 1, "32F EMA"},
		{"599 001", ExchangeUnknown, 2, "599 001"},
		{"599 05", ExchangeUnknown, 2, "599 05"},
		{"599 05", ExchangeRTTYRoundup, 1, "599 005"},
		{"599 0123", ExchangeRTTYRoundup, 1, "599 123"},
		{"K1ABC 599 001", ExchangeRTTYRoundup, 2, "K1ABC 599 001"},
	}
	for _, tc := range tests {
		frames, canonical, err := PackMessage(tc.message, tc.profile)
		if err != nil {
			t.Fatalf("%q profile=%d: %v", tc.message, tc.profile, err)
		}
		if len(frames) != tc.frames {
			t.Fatalf("%q profile=%d: frames=%d want %d", tc.message, tc.profile, len(frames), tc.frames)
		}
		if canonical != tc.canonical {
			t.Fatalf("%q profile=%d: canonical=%q want %q", tc.message, tc.profile, canonical, tc.canonical)
		}
		decoded, trailing, last, valid := UnpackMessage(frames)
		if !valid || !last {
			t.Fatalf("%q profile=%d: unpack valid=%v trailing=%v last=%v", tc.message, tc.profile, valid, trailing, last)
		}
		_ = trailing
		if decoded != tc.canonical {
			t.Fatalf("%q profile=%d: decoded=%q want %q", tc.message, tc.profile, decoded, tc.canonical)
		}
	}
}

func TestUnpackMessagePreservesText5SeparatorBoundary(t *testing.T) {
	frame, ok := packAtom(SourceAtom{Kind: atomText5, Text: "HELL "}, false)
	if !ok {
		t.Fatal("packAtom rejected text with trailing space")
	}
	got, trailing, last, valid := UnpackMessage([]PackedFrame{frame})
	if !valid || last {
		t.Fatalf("unpack valid=%v last=%v", valid, last)
	}
	if got != "HELL" || !trailing {
		t.Fatalf("got=%q trailing=%v, want %q with separator", got, trailing, "HELL")
	}
}

func TestPackAndAssembleLongTextKeepsFrameBoundarySpaces(t *testing.T) {
	message := "HELLO WORLD ABCDE FGHIJ KLMNO PQRST UVWXY 12345 67890 ABCDE FGHIJ KLMNO"
	frames, canonical, err := PackMessage(message, ExchangeUnknown)
	if err != nil {
		t.Fatalf("PackMessage: %v", err)
	}
	if len(frames) < 3 {
		t.Fatalf("frames=%d, want multiple frames", len(frames))
	}
	a := NewMessageAssembler()
	period := float64(nFrameSymbols*nSS) / fs6k
	var got string
	for i, packed := range frames {
		text, trailing, _, valid := UnpackMessage([]PackedFrame{packed})
		if !valid {
			t.Fatalf("frame %d failed to unpack", i)
		}
		updates, accepted := a.Add(FrameDecode{
			FrequencyHz: 1500,
			XDT:         float64(i) * period,
			Decoded:     text,
			TrailingSep: trailing,
			LastFrame:   i == len(frames)-1,
		}, period)
		if !accepted || len(updates) != 1 {
			t.Fatalf("frame %d accepted=%v updates=%#v", i, accepted, updates)
		}
		got = updates[0].Text
	}
	if got != canonical {
		t.Fatalf("assembled=%q want %q", got, canonical)
	}
}
