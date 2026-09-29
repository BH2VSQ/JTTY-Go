package decode

import (
	"github.com/BH2VSQ/jtty-go/internal/model"
	"testing"
	"time"
)

func TestStorePreservesAcceptanceOrder(t *testing.T) {
	s := NewStore(10)
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := s.Append(model.DecodeMessage{SignalUTC: late.Add(-2 * time.Second), ReceivedUTC: late, Message: "A"})
	second := s.Append(model.DecodeMessage{SignalUTC: late.Add(-3 * time.Second), ReceivedUTC: late.Add(time.Millisecond), Message: "B"})
	items := s.Snapshot()
	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("sequence mismatch: %d %d", first.Sequence, second.Sequence)
	}
	if items[0].Message != "A" || items[1].Message != "B" {
		t.Fatalf("store reordered late result: %#v", items)
	}
}

func TestStoreJTTYUpsertKeepsSequence(t *testing.T) {
	s := NewStore(10)
	first, added := s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 7, Message: "WB9XYZ"})
	if !added || first.Sequence != 1 {
		t.Fatalf("first=%#v added=%v", first, added)
	}
	second, added := s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 7, Message: "WB9XYZ 599 123"})
	if added || second.Sequence != first.Sequence {
		t.Fatalf("second=%#v added=%v", second, added)
	}
	third, added := s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 8, Message: "CQ K1ABC CQ"})
	if !added || third.Sequence != 2 {
		t.Fatalf("third=%#v added=%v", third, added)
	}
	got := s.Snapshot()
	if got[0].Message != "WB9XYZ 599 123" || got[1].Message != "CQ K1ABC CQ" {
		t.Fatalf("snapshot=%#v", got)
	}
}

func TestStoreRingOverwritePreservesNewestOrder(t *testing.T) {
	s := NewStore(3)
	for i := 1; i <= 5; i++ {
		s.Append(model.DecodeMessage{Message: string(rune('A' + i - 1))})
	}
	got := s.Snapshot()
	if len(got) != 3 || got[0].Message != "C" || got[1].Message != "D" || got[2].Message != "E" {
		t.Fatalf("snapshot=%#v", got)
	}
}

func TestStoreRingJTTYUpsertAfterOverwrite(t *testing.T) {
	s := NewStore(2)
	s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 1, Message: "A"})
	s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 2, Message: "B"})
	s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 3, Message: "C"})
	updated, added := s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 2, Message: "B2"})
	if added || updated.Message != "B2" || updated.Sequence != 2 {
		t.Fatalf("updated=%#v added=%v", updated, added)
	}
	missing, added := s.UpsertJTTY(model.DecodeMessage{JTTYMessageID: 1, Message: "A2"})
	if !added || missing.Sequence != 4 {
		t.Fatalf("evicted id unexpectedly updated: %#v added=%v", missing, added)
	}
}
