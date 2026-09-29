package decoder

import (
	"context"
	"sync"
	"time"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

type Job struct {
	CandidateID uint64
	Samples     []float32
	FrequencyHz float64
	Timestamp   time.Time
}

type Decoder interface {
	Decode(ctx context.Context, job Job) ([]model.DecodeMessage, error)
}

type Pool struct {
	jobs    chan Job
	results chan []model.DecodeMessage
	dec     Decoder
	wg      sync.WaitGroup
}

func NewPool(workers int, dec Decoder, queueSize int) *Pool {
	if workers < 1 {
		workers = 1
	}
	if queueSize < workers {
		queueSize = workers * 4
	}
	p := &Pool{jobs: make(chan Job, queueSize), results: make(chan []model.DecodeMessage, queueSize), dec: dec}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer p.wg.Done()
			for job := range p.jobs {
				msgs, err := p.dec.Decode(context.Background(), job)
				if err == nil && len(msgs) > 0 {
					p.results <- msgs
				}
			}
		}()
	}
	return p
}

func (p *Pool) Submit(job Job) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

func (p *Pool) Results() <-chan []model.DecodeMessage { return p.results }

func (p *Pool) Close() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}
