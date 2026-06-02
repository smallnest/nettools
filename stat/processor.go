package stat

import (
	"context"
	"sync"
	"time"
)

// Processor periodically calls statOnce on all registered Stat instances.
type Processor struct {
	mu    sync.Mutex
	stats []Stat
	span  time.Duration
	delay time.Duration
}

// NewProcessor creates a Processor that reports statistics every span,
// starting after an initial delay.
func NewProcessor(span, delay time.Duration) *Processor {
	return &Processor{
		delay: delay,
		span:  span,
	}
}

// AddStat registers a Stat instance for periodic reporting. Safe for concurrent use.
func (p *Processor) AddStat(s Stat) {
	p.mu.Lock()
	p.stats = append(p.stats, s)
	p.mu.Unlock()
}

// Run starts the periodic reporting loop. It waits for the configured
// delay, then reports all stats once per span tick. It stops when ctx is cancelled.
func (p *Processor) Run(ctx context.Context) {
	time.Sleep(p.delay)
	tick := time.NewTicker(p.span)
	defer tick.Stop()

	p.runOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.runOnce()
		}
	}
}

func (p *Processor) runOnce() {
	p.mu.Lock()
	snapshot := make([]Stat, len(p.stats))
	copy(snapshot, p.stats)
	p.mu.Unlock()

	for _, s := range snapshot {
		s.statOnce()
	}
}
