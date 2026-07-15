package topgg

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
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

	bucket   Bucket
	globalMu sync.RWMutex
}

type rateLimitTransport struct {
	limiter        *RateLimiter
	innerTransport http.RoundTripper

	retryCounter   atomic.Int64
	retryThreshold int64
	trippedUntil   atomic.Int64
	maxRetries     uint8
}

func NewRateLimiter(opt RateLimiterOptions) *RateLimiter {
	if opt.TraceLogger == nil {
		opt.TraceLogger = log.New(io.Discard, "", 0)
	}

	return &RateLimiter{
		traceLogger: opt.TraceLogger,
		bucket: Bucket{
			Limit:     100,
			Remaining: 100,
		},
	}
}

func (rl *RateLimiter) tracef(format string, v ...any) {
	rl.traceLogger.Printf("[(REST) HTTP RATE LIMITER] "+format, v...)
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.globalMu.RLock()
	if !rl.globalWait.IsZero() && time.Now().Before(rl.globalWait) {
		wait := time.Until(rl.globalWait)
		rl.globalMu.RUnlock()
		rl.tracef("Global rate limit or 429 hit! Waiting %s...", wait.Round(time.Millisecond))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	} else {
		rl.globalMu.RUnlock()
	}

	for {
		rl.bucket.mu.Lock()
		now := time.Now()

		if now.After(rl.bucket.ResetAt) {
			rl.bucket.Remaining = rl.bucket.Limit
			rl.bucket.ResetAt = now.Add(time.Second)
		}

		if rl.bucket.Remaining > 0 {
			rl.bucket.Remaining--
			rl.bucket.mu.Unlock()
			return nil
		}

		waitDuration := rl.bucket.ResetAt.Sub(now)
		rl.bucket.mu.Unlock()

		rl.tracef("Rate limit hit on global bucket! Waiting %s...", waitDuration.Round(time.Millisecond))

		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (rl *RateLimiter) SetGlobalWait(d time.Duration) {
	rl.globalMu.Lock()
	defer rl.globalMu.Unlock()

	rl.globalWait = time.Now().Add(d)
	rl.tracef("Received 429! All requests suspended for %s", d.Round(time.Millisecond))
}

func (t *rateLimitTransport) tripped() bool {
	return t.trippedUntil.Load() > time.Now().UnixNano()
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		lastResp *http.Response
		lastErr  error
		retrying bool
	)

	defer func() {
		if retrying {
			t.retryCounter.Add(-1)
		}
	}()

	for i := uint8(0); i < t.maxRetries; i++ {
		if t.tripped() {
			return nil, ErrLocalRatelimit
		}

		if err := t.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}

		if i > 0 && req.Body != nil {
			if req.GetBody == nil {
				if lastResp != nil {
					return nil, lastErr
				}
				return nil, fmt.Errorf("request failed after %d retries (body is not rewindable): %w", i, lastErr)
			}
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to get request body for retry: %w", err)
			}

			req.Body = body
		}

		resp, err := t.innerTransport.RoundTrip(req)
		if err != nil {
			lastErr = err

			if !retrying {
				retrying = true
				if t.retryCounter.Add(1) > t.retryThreshold {
					t.trippedUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
					return nil, fmt.Errorf("%w: global retry threshold (%d) exceeded", ErrLocalRatelimit, t.retryThreshold)
				}
			}

			if i < t.maxRetries-1 {
				timer := time.NewTimer(time.Millisecond * time.Duration(250*int64(i+1)))
				select {
				case <-req.Context().Done():
					timer.Stop()
					return nil, req.Context().Err()
				case <-timer.C:
				}
			}

			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if !retrying {
				retrying = true
				if t.retryCounter.Add(1) > t.retryThreshold {
					t.trippedUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
					errRet := fmt.Errorf("%w: global retry threshold (%d) exceeded", ErrLocalRatelimit, t.retryThreshold)
					if cErr := resp.Body.Close(); cErr != nil {
						errRet = fmt.Errorf("%w, and failed to close response body: %v", errRet, cErr)
					}

					return nil, errRet
				}
			}

			if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
				if retryAfterSec, err := strconv.Atoi(retryAfterStr); err == nil {
					t.limiter.SetGlobalWait(time.Duration(retryAfterSec) * time.Second)
				}
			} else {
				t.limiter.SetGlobalWait(time.Minute)
			}

			lastResp = resp
			lastErr = ErrRemoteRatelimit
			if cErr := resp.Body.Close(); cErr != nil {
				lastErr = fmt.Errorf("%w, and failed to close response body: %v", lastErr, cErr)
			}

			if i < t.maxRetries-1 {
				timer := time.NewTimer(time.Millisecond * time.Duration(250*int64(i+1)))
				select {
				case <-req.Context().Done():
					timer.Stop()
					return nil, req.Context().Err()
				case <-timer.C:
				}
			}

			continue
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			if !retrying {
				retrying = true
				if t.retryCounter.Add(1) > t.retryThreshold {
					t.trippedUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
					errRet := fmt.Errorf("%w: global retry threshold (%d) exceeded", ErrLocalRatelimit, t.retryThreshold)
					if cErr := resp.Body.Close(); cErr != nil {
						errRet = fmt.Errorf("%w, and failed to close response body: %v", errRet, cErr)
					}

					return nil, errRet
				}
			}

			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				lastErr = fmt.Errorf("%w: failed to read top.gg API internal server error body: %v", ErrRequestFailed, err)
			} else {
				lastErr = fmt.Errorf("%w: top.gg API internal server error: %s", ErrRequestFailed, resp.Status)
			}

			lastResp = resp
			if cErr := resp.Body.Close(); cErr != nil {
				lastErr = fmt.Errorf("%w, and failed to close response body: %v", lastErr, cErr)
			}

			if i < t.maxRetries-1 {
				timer := time.NewTimer(time.Millisecond * time.Duration(250*int64(i+1)))
				select {
				case <-req.Context().Done():
					timer.Stop()
					return nil, req.Context().Err()
				case <-timer.C:
				}
			}

			continue
		}

		return resp, nil
	}

	if lastErr != nil {
		if lastResp != nil {
			return nil, lastErr
		}

		return nil, fmt.Errorf("request failed after %d retries: %w", t.maxRetries, lastErr)
	}

	return lastResp, nil
}
