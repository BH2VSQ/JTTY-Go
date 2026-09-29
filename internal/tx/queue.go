package tx

import "sync"

type Queue struct {
	mu    sync.Mutex
	items []string
}

func NewQueue() *Queue { return &Queue{} }

func (q *Queue) Push(message string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, message)
}

func (q *Queue) Pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *Queue) Snapshot() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.items...)
}

func (q *Queue) Clear() {
	q.mu.Lock()
	q.items = nil
	q.mu.Unlock()
}
