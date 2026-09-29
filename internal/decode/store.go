package decode

import (
	"sync"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

// Store owns presentation ordering. Sequence is assigned only when a new live
// JTTY message identity is first accepted. Later frame updates replace the
// existing row and never reorder it.
//
// The backing slice is a fixed-size ring once it reaches the configured limit.
// This avoids copying the entire decode history on every new message after the
// limit is reached and keeps long-running sessions from creating unnecessary
// GC pressure.
type Store struct {
	mu    sync.RWMutex
	limit int
	next  uint64
	items []model.DecodeMessage
	head  int
	size  int
	byID  map[uint64]int
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = 2000
	}
	return &Store{limit: limit, items: make([]model.DecodeMessage, 0, limit), byID: make(map[uint64]int)}
}

func (s *Store) Append(item model.DecodeMessage) model.DecodeMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendLocked(item)
}

// UpsertJTTY updates an existing JTTY message in place when its stable
// message ID is non-zero. A new ID is appended at the tail. This mirrors the
// message_update identity used by WSJT-X while preserving our stronger UI
// rule: newly accepted messages always appear at the bottom.
func (s *Store) UpsertJTTY(item model.DecodeMessage) (model.DecodeMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.JTTYMessageID == 0 {
		return s.appendLocked(item), true
	}
	if idx, ok := s.byID[item.JTTYMessageID]; ok && idx >= 0 && idx < len(s.items) {
		item.Sequence = s.items[idx].Sequence
		s.items[idx] = item
		return item, false
	}
	return s.appendLocked(item), true
}

func (s *Store) appendLocked(item model.DecodeMessage) model.DecodeMessage {
	s.next++
	item.Sequence = s.next

	if s.size < s.limit {
		idx := len(s.items)
		s.items = append(s.items, item)
		s.size++
		if item.JTTYMessageID != 0 {
			s.byID[item.JTTYMessageID] = idx
		}
		return item
	}

	// Full ring: overwrite the oldest entry and advance head. No whole-history
	// allocation or copy is necessary.
	idx := s.head
	old := s.items[idx]
	if old.JTTYMessageID != 0 {
		delete(s.byID, old.JTTYMessageID)
	}
	s.items[idx] = item
	if item.JTTYMessageID != 0 {
		s.byID[item.JTTYMessageID] = idx
	}
	s.head = (s.head + 1) % s.limit
	return item
}

// SetLimit updates the rolling decode history limit while preserving the
// newest entries. Rebuilding only occurs when the configured capacity changes.
func (s *Store) SetLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.limit == limit {
		return
	}
	current := s.snapshotLocked()
	if len(current) > limit {
		current = current[len(current)-limit:]
	}
	s.limit = limit
	s.items = make([]model.DecodeMessage, len(current), limit)
	copy(s.items, current)
	s.head = 0
	s.size = len(current)
	s.byID = make(map[uint64]int, len(current))
	for i, item := range s.items {
		if item.JTTYMessageID != 0 {
			s.byID[item.JTTYMessageID] = i
		}
	}
}

func (s *Store) snapshotLocked() []model.DecodeMessage {
	out := make([]model.DecodeMessage, 0, s.size)
	if s.size == 0 {
		return out
	}
	for i := 0; i < s.size; i++ {
		idx := (s.head + i) % len(s.items)
		out = append(out, s.items[idx])
	}
	return out
}

func (s *Store) Snapshot() []model.DecodeMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *Store) Len() int { s.mu.RLock(); defer s.mu.RUnlock(); return s.size }
