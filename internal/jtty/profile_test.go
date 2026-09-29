package jtty

import "testing"

func TestWSJTXDefaultProfiles(t *testing.T) {
	want := []string{
		"CQ %M CQ",
		"%H %E",
		"%H TU CQ %M CQ",
		"%M",
		"%H",
		"TU NOW %Q %E",
		"%H AGN?",
		"%E",
	}
	if len(WSJTXDefaultProfiles) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(WSJTXDefaultProfiles), len(want))
	}
	for i, profile := range WSJTXDefaultProfiles {
		if profile.Macro != want[i] {
			t.Fatalf("profile %d: got %q, want %q", i+1, profile.Macro, want[i])
		}
	}
}

func TestExpandExchange(t *testing.T) {
	if got := ExpandExchange(107); got != "599 107" {
		t.Fatalf("got %q, want %q", got, "599 107")
	}
	if got := ExpandExchange(1); got != "599 001" {
		t.Fatalf("got %q, want %q", got, "599 001")
	}
}

func TestSplitArbitraryText(t *testing.T) {
	frames := SplitArbitraryText("1234567890AB")
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3", len(frames))
	}
	if frames[0].Text != "12345" || frames[1].Text != "67890" || frames[2].Text != "AB" {
		t.Fatalf("unexpected frames: %#v", frames)
	}
}
