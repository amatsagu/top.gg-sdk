package topgg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type Client struct {
	limiter        *RateLimiter
	traceLogger    *log.Logger
	HTTPClient     http.Client
	token          string
	maxWaitTime    time.Duration
	retryCounter   atomic.Int64
	retryThreshold int64
	trippedUntil   atomic.Int64
	maxRetries     uint8
}

type ClientOptions struct {
	RateLimiterOptions RateLimiterOptions
	Token              string
	MaxWaitTime        time.Duration
	RetryThreshold     uint32
	MaxRetries         uint8
	Trace              bool
}

type rateLimitError struct {
	RetryAfter float64 `json:"retry-after"`
}

func NewClient(opt ClientOptions) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: false,
		MaxIdleConns:      256,
		IdleConnTimeout:   90 * time.Second,
	}

	maxTimeout := 10 * time.Second
	if opt.MaxWaitTime != 0 {
		maxTimeout = opt.MaxWaitTime
	}

	maxRetries := opt.MaxRetries
	if opt.MaxRetries == 0 {
		maxRetries = 3
	}

	retryThreshold := opt.RetryThreshold
	if retryThreshold == 0 {
		retryThreshold = 60
	}

	var traceLogger *log.Logger
	if opt.Trace {
		traceLogger = log.New(os.Stdout, "", log.LstdFlags)
		opt.RateLimiterOptions.TraceLogger = traceLogger
	}

	limiter := NewRateLimiter(opt.RateLimiterOptions)

	return &Client{
		limiter: limiter,
		HTTPClient: http.Client{
			Transport: &rateLimitTransport{
				limiter:        limiter,
				innerTransport: transport,
			},
			Timeout: maxTimeout,
		},
		token:          opt.Token,
		maxRetries:     maxRetries,
		maxWaitTime:    maxTimeout,
		retryThreshold: int64(retryThreshold),
		traceLogger:    traceLogger,
	}
}

func (c *Client) tripped() bool {
	return c.trippedUntil.Load() > time.Now().UnixNano()
}

func (c *Client) request(method, route string, jsonPayload any) ([]byte, error) {
	var body io.ReadSeeker
	if jsonPayload != nil {
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(jsonPayload); err != nil {
			return nil, fmt.Errorf("failed to encode JSON payload: %w", err)
		}
		body = bytes.NewReader(buf.Bytes())
	}

	var (
		responseBody []byte
		lastErr      error
		retrying     bool
	)

	defer func() {
		if retrying {
			c.retryCounter.Add(-1)
		}
	}()

	for i := uint8(0); i < c.maxRetries; i++ {
		if body != nil {
			if _, err := body.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek request body: %w", err)
			}
		}

		if c.tripped() {
			return nil, ErrLocalRatelimit
		}

		url := BaseURL + route
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		req.Header.Set("Authorization", "Bearer "+c.token)

		responseBody, err = c.executeOnce(req)
		if err != nil {
			lastErr = err

			if errors.Is(err, ErrRemoteRatelimit) || errors.Is(err, ErrRequestFailed) {
				if !retrying {
					retrying = true
					if c.retryCounter.Add(1) > c.retryThreshold {
						c.trippedUntil.Store(time.Now().Add(5 * time.Second).UnixNano())
						return nil, fmt.Errorf("%w: global retry threshold (%d) exceeded", ErrLocalRatelimit, c.retryThreshold)
					}
				}

				if errors.Is(err, ErrRemoteRatelimit) {
					continue
				}

				time.Sleep(time.Millisecond * time.Duration(250*int64(i+1)))
				continue
			}

			return nil, err
		}
		return responseBody, nil
	}

	return nil, fmt.Errorf("request failed after %d retries on %s %s: %w", c.maxRetries, method, route, lastErr)
}

func (c *Client) executeOnce(req *http.Request) ([]byte, error) {
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: http request execution failed: %s", ErrRequestFailed, err.Error())
	}
	defer func() { _ = res.Body.Close() }() // Thanks linter...

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices {
		return responseBody, nil
	}

	if res.StatusCode == http.StatusTooManyRequests {
		var rateErr rateLimitError
		err = json.Unmarshal(responseBody, &rateErr)
		if err != nil {
			return nil, fmt.Errorf("failed to read response rate limited error body: %w", err)
		}

		retryAfter := time.Duration(rateErr.RetryAfter * float64(time.Second))
		if retryAfter == 0 {
			retryAfter = time.Minute // Safe default
		}

		c.limiter.SetGlobalWait(retryAfter)
		return nil, fmt.Errorf("%w: retrying after %s", ErrRemoteRatelimit, retryAfter.String())
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorizedRequest
	}

	if res.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: top.gg API internal server error: %s", ErrRequestFailed, res.Status)
	}

	return nil, fmt.Errorf("request failed with status %s: %s", res.Status, string(responseBody))
}

func (c *Client) NewWebhookHandler(opt WebhookOptions) http.Handler {
	window := opt.TimestampWindow
	if window == 0 {
		window = 30 * time.Second
	}

	return &Webhook{
		secret:              opt.Secret,
		timestampWindow:     window,
		client:              c,
		onVote:              opt.OnVote,
		onIntegrationCreate: opt.OnIntegrationCreate,
		onIntegrationDelete: opt.OnIntegrationDelete,
		onTest:              opt.OnTest,
	}
}
