package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter keeps one token bucket per key (usually a client IP). Idle buckets
// are dropped so the map cannot grow without bound.
type Limiter struct {
	mu      sync.Mutex
	every   rate.Limit
	burst   int
	buckets map[string]*bucket
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New allows burst requests at once and then one every interval.
func New(interval time.Duration, burst int) *Limiter {
	return &Limiter{every: rate.Every(interval), burst: burst, buckets: map[string]*bucket{}}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.buckets) > 10000 {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.every, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = now
	return b.limiter.AllowN(now, 1)
}

func (l *Limiter) sweep(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > time.Hour {
			delete(l.buckets, key)
		}
	}
}
