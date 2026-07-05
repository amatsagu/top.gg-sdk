package dbl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// https://docs.top.gg/api/v1/webhooks#supported-scopes
type Scope string

const (
	ScopeVoteCreate        Scope = "vote.create"
	ScopeWebhookTest       Scope = "webhook.test"
	ScopeIntegrationCreate Scope = "integration.create"
	ScopeIntegrationDelete Scope = "integration.delete"
)

// Represents a minimal project object from a webhook payload
type PartialProject struct {
	Type       ProjectType `json:"type"`
	Platform   Platform    `json:"platform"`
	ID         Snowflake   `json:"id"`
	PlatformID Snowflake   `json:"platform_id"`
}

// WebhookUser represents a user in a webhook payload
type WebhookUser struct {
	Name       string    `json:"name"`
	Avatar     string    `json:"avatar_url"`
	ID         Snowflake `json:"id"`
	PlatformID Snowflake `json:"platform_id"`
}

type IntegrationCreatePayload struct {
	ConnectionID string         `json:"connection_id"`
	Secret       string         `json:"webhook_secret"`
	Project      PartialProject `json:"project"`
	User         WebhookUser    `json:"user"`
}

type IntegrationDeletePayload struct {
	ConnectionID string `json:"connection_id"`
}

// https://docs.top.gg/webhooks/events#vote-create
type VoteCreatePayload struct {
	VotedAt   time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	Query     map[string]string `json:"query"`
	Project   PartialProject    `json:"project"`
	User      WebhookUser       `json:"user"`
	ID        Snowflake         `json:"id"`
	Weight    int               `json:"weight"`
}

// https://docs.top.gg/webhooks/events#webhook-test
type WebhookTestPayload struct {
	Project PartialProject `json:"project"`
	User    WebhookUser    `json:"user"`
}

type WebhookPayload struct {
	Type  Scope           `json:"type"`
	Trace string          `json:"trace,omitempty"` // populated from x-topgg-trace header
	Data  json.RawMessage `json:"data"`
}

type WebhookOptions struct {
	OnVote              func(vote VoteCreatePayload)
	OnIntegrationCreate func(integration IntegrationCreatePayload)
	OnIntegrationDelete func(integration IntegrationDeletePayload)
	OnTest              func(test WebhookTestPayload)
	Secret              string
	TimestampWindow     time.Duration
}

// https://docs.top.gg/webhooks/events#legacy-v0-webhook-events
type v0VotePayload struct {
	Type      string    `json:"type"`
	Query     string    `json:"query"`
	Bot       Snowflake `json:"bot"`
	User      Snowflake `json:"user"`
	IsWeekend bool      `json:"isWeekend"`
}

type Webhook struct {
	client              *Client
	onVote              func(vote VoteCreatePayload)
	onIntegrationCreate func(integration IntegrationCreatePayload)
	onIntegrationDelete func(integration IntegrationDeletePayload)
	onTest              func(test WebhookTestPayload)
	secret              string
	timestampWindow     time.Duration
}

// Automatically handles both v0 (legacy Authorization) and v1 (x-topgg-signature HMAC) webhooks.
func (w *Webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024)) // 2MB limit
	if err != nil {
		http.Error(rw, "bad request", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	signatureHeader := r.Header.Get("x-topgg-signature")
	if signatureHeader != "" {
		w.handleV1(rw, body, signatureHeader)
	} else {
		w.handleV0(rw, body, r.Header.Get("Authorization"))
	}
}

// Parses modern v1 Webhooks using HMAC verification and routes to callbacks.
// The integration secret will automatically update in-memory upon integration.create events.
func (w *Webhook) handleV1(rw http.ResponseWriter, body []byte, signatureHeader string) {
	if err := w.validateV1(signatureHeader, body); err != nil {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(rw, "bad request", http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case ScopeVoteCreate:
		if w.onVote != nil {
			var vote VoteCreatePayload
			if err := json.Unmarshal(payload.Data, &vote); err == nil {
				w.onVote(vote)
			}
		}
	case ScopeIntegrationCreate:
		var integration IntegrationCreatePayload
		if err := json.Unmarshal(payload.Data, &integration); err == nil {
			w.secret = integration.Secret // Auto-update secret
			if w.onIntegrationCreate != nil {
				w.onIntegrationCreate(integration)
			}
		}
	case ScopeIntegrationDelete:
		if w.onIntegrationDelete != nil {
			var integration IntegrationDeletePayload
			if err := json.Unmarshal(payload.Data, &integration); err == nil {
				w.onIntegrationDelete(integration)
			}
		}
	case ScopeWebhookTest:
		if w.onTest != nil {
			var test WebhookTestPayload
			if err := json.Unmarshal(payload.Data, &test); err == nil {
				w.onTest(test)
			}
		}
	}

	rw.WriteHeader(http.StatusOK)
}

// Parses legacy Webhooks using raw string Authorization match.
// It normalizes v0 webhook payloads into the v1 VoteCreatePayload structure to ensure callback consistency.
func (w *Webhook) handleV0(rw http.ResponseWriter, body []byte, authHeader string) {
	if authHeader != w.secret {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	var v0Payload v0VotePayload
	if err := json.Unmarshal(body, &v0Payload); err != nil {
		http.Error(rw, "bad request", http.StatusBadRequest)
		return
	}

	if v0Payload.Type == "test" {
		if w.onTest != nil {
			w.onTest(WebhookTestPayload{
				Project: PartialProject{
					ID:   v0Payload.Bot,
					Type: ProjectTypeBot,
				},
				User: WebhookUser{
					ID: v0Payload.User,
				},
			})
		}
	} else if w.onVote != nil {
		weight := 1
		if v0Payload.IsWeekend {
			weight = 2
		}

		queryMap := make(map[string]string)
		if v0Payload.Query != "" {
			qStr := strings.TrimPrefix(v0Payload.Query, "?")
			if parsed, err := url.ParseQuery(qStr); err == nil {
				for k, v := range parsed {
					if len(v) > 0 {
						queryMap[k] = v[0]
					}
				}
			}
		}

		now := time.Now().UTC()
		w.onVote(VoteCreatePayload{
			Weight:    weight,
			VotedAt:   now,
			ExpiresAt: now.Add(12 * time.Hour), // roughly standard vote expiration
			Project: PartialProject{
				ID:   v0Payload.Bot,
				Type: ProjectTypeBot,
			},
			User: WebhookUser{
				ID: v0Payload.User,
			},
			Query: queryMap,
		})
	}

	rw.WriteHeader(http.StatusOK)
}

func (w *Webhook) validateV1(signatureHeader string, body []byte) error {
	parts := strings.Split(signatureHeader, ",")
	parsedSignature := make(map[string]string)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			parsedSignature[kv[0]] = kv[1]
		}
	}

	tStr, hasT := parsedSignature["t"]
	sig, hasSig := parsedSignature["v1"]

	if !hasT || !hasSig {
		return fmt.Errorf("invalid signature format")
	}

	tInt, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format")
	}

	// Replay attack prevention
	tTime := time.Unix(tInt, 0)
	if w.timestampWindow > 0 {
		if time.Since(tTime) > w.timestampWindow || time.Since(tTime) < -w.timestampWindow {
			return fmt.Errorf("timestamp outside of accepted time window")
		}
	}

	mac := hmac.New(sha256.New, []byte(w.secret))
	_, _ = fmt.Fprintf(mac, "%s.", tStr) // Thanks linter...
	mac.Write(body)
	digest := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(digest)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
