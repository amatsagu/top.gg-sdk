package topgg

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Bucket struct {
	ResetAt   time.Time
	Remaining int
	Limit     int
	mu        sync.Mutex
}

type RateLimiterOptions struct {
	TraceLogger *log.Logger
}

type RateLimiter struct {
	globalWait  time.Time
	traceLogger *log.Logger

	buckets  map[string]*Bucket
	mu       sync.RWMutex
	globalMu sync.RWMutex
}

type rateLimitTransport struct {
	limiter        *RateLimiter
	innerTransport http.RoundTripper
}

func NewRateLimiter(opt RateLimiterOptions) *RateLimiter {
	rl := &RateLimiter{
		buckets:     make(map[string]*Bucket),
		traceLogger: opt.TraceLogger,
	}

	rl.buckets["global"] = &Bucket{Limit: 100, Remaining: 100}
	rl.buckets["/bots"] = &Bucket{Limit: 60, Remaining: 60}
	rl.buckets["/users"] = &Bucket{Limit: 60, Remaining: 60}

	return rl
}

func (rl *RateLimiter) tracef(format string, v ...any) {
	if rl.traceLogger != nil {
		rl.traceLogger.Printf("[(REST) LIMITER] "+format, v...)
	}
}

func (rl *RateLimiter) Wait(route string) func() {
	rl.globalMu.RLock()
	if !rl.globalWait.IsZero() && time.Now().Before(rl.globalWait) {
		wait := time.Until(rl.globalWait)
		rl.globalMu.RUnlock()
		rl.tracef("Global rate limit or 429 hit! Waiting %s...", wait.Round(time.Millisecond))
		time.Sleep(wait)
	} else {
		rl.globalMu.RUnlock()
	}

	bucketKey := "global"
	if strings.HasPrefix(route, "/bots") {
		bucketKey = "/bots"
	} else if strings.HasPrefix(route, "/users") {
		bucketKey = "/users"
	}

	rl.mu.Lock()
	bucket, exists := rl.buckets[bucketKey]
	if !exists {
		bucket = rl.buckets["global"]
	}
	rl.mu.Unlock()

	bucket.mu.Lock()
	now := time.Now()

	if now.After(bucket.ResetAt) {
		bucket.Remaining = bucket.Limit
		if bucketKey == "global" {
			bucket.ResetAt = now.Add(time.Second)
		} else {
			bucket.ResetAt = now.Add(time.Minute)
		}
	}

	if bucket.Remaining <= 0 {
		waitDuration := bucket.ResetAt.Sub(now)
		rl.tracef("Rate limit hit on bucket \"%s\"! Waiting %s...", bucketKey, waitDuration.Round(time.Millisecond))
		time.Sleep(waitDuration)
		bucket.Remaining = bucket.Limit
		if bucketKey == "global" {
			bucket.ResetAt = time.Now().Add(time.Second)
		} else {
			bucket.ResetAt = time.Now().Add(time.Minute)
		}
	}

	bucket.Remaining--
	return func() {
		bucket.mu.Unlock()
	}
}

func (rl *RateLimiter) SetGlobalWait(d time.Duration) {
	rl.globalMu.Lock()
	defer rl.globalMu.Unlock()
	rl.globalWait = time.Now().Add(d)
	rl.tracef("Received 429! All requests suspended for %s", d.Round(time.Millisecond))
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	route := req.URL.Path
	if strings.HasPrefix(route, "/api/v0") {
		route = route[7:]
	} else if strings.HasPrefix(route, "/api/v1") {
		route = route[7:]
	}

	unlock := t.limiter.Wait(route)

	resp, err := t.innerTransport.RoundTrip(req)
	if err != nil {
		unlock()
		return nil, err
	}

	unlock()
	return resp, nil
}
