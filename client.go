package topgg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

type Client struct {
	limiter     *RateLimiter
	traceLogger *log.Logger
	HTTPClient  http.Client
	token       string
}

type ClientOptions struct {
	RateLimiterOptions RateLimiterOptions
	Token              string
	MaxWaitTime        time.Duration
	RetryThreshold     uint32
	MaxRetries         uint8
	Trace              bool
	HTTPClient         *http.Client // Optional client You want to use for making all API requests.
}

func NewClient(opt ClientOptions) *Client {
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

	clientCopy := http.Client{}
	if opt.HTTPClient != nil {
		clientCopy = *opt.HTTPClient
	} else {
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

		clientCopy.Transport = &rateLimitTransport{
			limiter:        limiter,
			innerTransport: transport,
			maxRetries:     maxRetries,
			retryThreshold: int64(retryThreshold),
		}
		clientCopy.Timeout = maxTimeout
	}

	return &Client{
		limiter:     limiter,
		HTTPClient:  clientCopy,
		token:       opt.Token,
		traceLogger: traceLogger,
	}
}

func (c *Client) request(ctx context.Context, method, route string, jsonPayload any) ([]byte, error) {
	var body io.Reader
	if jsonPayload != nil {
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(jsonPayload); err != nil {
			return nil, fmt.Errorf("failed to encode JSON payload: %w", err)
		}
		body = &buf
	}

	url := BaseURL + route
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices {
		return responseBody, nil
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorizedRequest
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
		onVote:              opt.OnVote,
		onIntegrationCreate: opt.OnIntegrationCreate,
		onIntegrationDelete: opt.OnIntegrationDelete,
		onTest:              opt.OnTest,
	}
}
