package main

import (
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }
type Limiter struct {
	mu     sync.Mutex
	clock  Clock
	rate   float64
	burst  int
	tokens float64
	last   time.Time
}

func NewLimiter(clock Clock, ratePerSec float64, burst int) *Limiter {
	limiter := &Limiter{
		clock:  clock,
		rate:   ratePerSec,
		burst:  burst,
		tokens: float64(burst),
	}

	if clock != nil && burst > 0 {
		limiter.last = clock.Now()
	}

	return limiter
}

func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.burst <= 0 || l.clock == nil {
		return false
	}

	if n <= 0 || n > l.burst {
		return false
	}

	if !l.clock.Now().Before(l.last) {
		timeDelta := l.clock.Now().Sub(l.last)
		if l.rate > 0 {
			l.tokens += l.rate * timeDelta.Seconds()
			if l.tokens > float64(l.burst) {
				l.tokens = float64(l.burst)
			}

			l.last = l.clock.Now()
		}

		if l.tokens < float64(n) {
			return false
		}

		l.tokens -= float64(n)
		return true
	}

	return false

}
