package dbl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// https://docs.top.gg/api/v1/projects#get-%2Fprojects%2Fproject_id
type Project struct {
	ID          Snowflake `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Type        string    `json:"type"`
	Headline    string    `json:"headline"`
	Tags        []string  `json:"tags,omitzero"`
	Votes       int       `json:"votes"`
	VotesTotal  int       `json:"votes_total"`
	ReviewScore float64   `json:"review_score"`
	ReviewCount int       `json:"review_count"`
}

type ProjectPayload struct {
	Headline    map[string]string `json:"headline,omitempty"`
	PageContent map[string]string `json:"page_content,omitempty"`
}

type Announcement struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type PartialVote struct {
	VotedAt   time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Weight    int       `json:"weight"`
}

type Vote struct {
	PartialVote
	VoterID    Snowflake `json:"user_id"`
	PlatformID Snowflake `json:"platform_id"`
}

type PaginatedVotes struct {
	Votes  []Vote
	Cursor string
}

type MetricsPayload struct {
	ServerCount int `json:"server_count,omitempty"`
	ShardCount  int `json:"shard_count,omitempty"`
	MemberCount int `json:"member_count,omitempty"`
	OnlineCount int `json:"online_count,omitempty"`
	PlayerCount int `json:"player_count,omitempty"`
}

type BatchMetricsPayload struct {
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Metrics   MetricsPayload `json:"metrics"`
}

// GetProject fetches project data by its ID.
// https://docs.top.gg/api/v1/projects#get-%2Fprojects%2Fproject_id
func (c *Client) GetProject(id Snowflake) (*Project, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v1/projects/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var project Project
	err = json.Unmarshal(b, &project)
	return &project, err
}

// https://docs.top.gg/api/v1/projects#get-/projects/@me
func (c *Client) GetMyProject() (*Project, error) {
	b, err := c.request(http.MethodGet, "/v1/projects/@me", nil)
	if err != nil {
		return nil, err
	}

	var project Project
	err = json.Unmarshal(b, &project)
	return &project, err
}

// https://docs.top.gg/api/v1/projects#patch-/projects/@me
func (c *Client) EditMyProject(payload ProjectPayload) error {
	_, err := c.request(http.MethodPatch, "/v1/projects/@me", payload)
	return err
}

// https://docs.top.gg/api/v1/projects#put-/projects/@me/commands
func (c *Client) PostApplicationCommands(commands []any) error {
	_, err := c.request(http.MethodPut, "/v1/projects/@me/commands", commands)
	return err
}

// https://docs.top.gg/api/v1/projects#post-/projects/@me/announcements
func (c *Client) PostAnnouncement(title, content, category string) (*Announcement, error) {
	body := map[string]string{
		"title":   title,
		"content": content,
	}
	if category != "" {
		body["category"] = category
	}

	b, err := c.request(http.MethodPost, "/v1/projects/@me/announcements", body)
	if err != nil {
		return nil, err
	}

	var announcement Announcement
	err = json.Unmarshal(b, &announcement)
	return &announcement, err
}

// https://docs.top.gg/api/v1/projects#patch-/projects/@me/metrics
func (c *Client) PostMyMetrics(payload MetricsPayload) error {
	_, err := c.request(http.MethodPatch, "/v1/projects/@me/metrics", payload)
	return err
}

// https://docs.top.gg/api/v1/projects#post-/projects/@me/metrics/batch
func (c *Client) PostMyMetricsInBatch(payload []BatchMetricsPayload) error {
	body := map[string]any{"data": payload}
	_, err := c.request(http.MethodPost, "/v1/projects/@me/metrics/batch", body)
	return err
}

// https://docs.top.gg/api/v1/votes#get-/projects/@me/votes/user_id
func (c *Client) GetVote(userID Snowflake, source string) (*PartialVote, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v1/projects/@me/votes/%s?source=%s", userID, source), nil)
	if err != nil {
		return nil, err
	}

	var vote PartialVote
	err = json.Unmarshal(b, &vote)
	return &vote, err
}

// https://docs.top.gg/api/v1/votes#get-/projects/@me/votes
func (c *Client) GetVotes(cursor string, startDate *time.Time) (*PaginatedVotes, error) {
	q := url.Values{}
	if startDate != nil {
		q.Set("startDate", startDate.Format(time.RFC3339))
	} else if cursor != "" {
		q.Set("cursor", cursor)
	}

	b, err := c.request(http.MethodGet, fmt.Sprintf("/v1/projects/@me/votes?%s", q.Encode()), nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Data   []Vote `json:"data"`
		Cursor string `json:"cursor"`
	}

	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}

	return &PaginatedVotes{
		Votes:  res.Data,
		Cursor: res.Cursor,
	}, nil
}
