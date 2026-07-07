package topgg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// https://docs.top.gg/api/v1/projects#get-%2Fprojects%2Fproject_id
type Project struct {
	Name        string      `json:"name"`
	Platform    Platform    `json:"platform"`
	Type        ProjectType `json:"type"`
	Headline    string      `json:"headline"`
	Tags        []string    `json:"tags"`
	ID          Snowflake   `json:"id"`
	Votes       int         `json:"votes"`
	VotesTotal  int         `json:"votes_total"`
	ReviewScore float64     `json:"review_score"`
	ReviewCount int         `json:"review_count"`
}

// https://docs.top.gg/api/v1/projects#request-body
type ProjectPayload struct {
	Headline    map[Locale]string `json:"headline,omitempty"`
	PageContent map[Locale]string `json:"page_content,omitempty"`
}

// https://docs.top.gg/api/v1/projects#response-fields-2
type Announcement struct {
	CreatedAt time.Time `json:"created_at"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
}

// https://docs.top.gg/api/v1/votes#response-fields-2
type PartialVote struct {
	VotedAt   time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Weight    int       `json:"weight"`
}

// https://docs.top.gg/api/v1/votes#param-data
type Vote struct {
	PartialVote
	VoterID    Snowflake `json:"user_id"`
	PlatformID Snowflake `json:"platform_id"`
}

// https://docs.top.gg/api/v1/votes#response-fields
type PaginatedVotes struct {
	Cursor string
	Votes  []Vote
}

// https://docs.top.gg/api/v1/projects#discord-server
// https://docs.top.gg/api/v1/projects#roblox-game
type MetricsPayload struct {
	ServerCount int `json:"server_count,omitempty"`
	ShardCount  int `json:"shard_count,omitempty"`
	MemberCount int `json:"member_count,omitempty"`
	OnlineCount int `json:"online_count,omitempty"`
	PlayerCount int `json:"player_count,omitempty"`
}

// https://docs.top.gg/api/v1/projects#param-platform
type Platform string

const (
	PlatformDiscord Platform = "discord"
	PlatformRoblox  Platform = "roblox"
)

// https://docs.top.gg/api/v1/projects#param-type
type ProjectType string

const (
	ProjectTypeBot    ProjectType = "bot"
	ProjectTypeServer ProjectType = "server"
	ProjectTypeGame   ProjectType = "game"
)

// https://docs.top.gg/api/v1/projects#supported-locales
type Locale string

const (
	LocaleEnglish    Locale = "en"
	LocaleGerman     Locale = "de"
	LocaleFrench     Locale = "fr"
	LocalePortuguese Locale = "pt"
	LocaleTurkish    Locale = "tr"
	LocaleHindi      Locale = "hi"
	LocaleJapanese   Locale = "ja"
	LocaleArabic     Locale = "ar"
	LocaleDutch      Locale = "nl"
	LocaleKorean     Locale = "ko"
	LocaleItalian    Locale = "it"
	LocaleSpanish    Locale = "es"
	LocaleRussian    Locale = "ru"
	LocaleUkrainian  Locale = "uk"
	LocaleVietnamese Locale = "vi"
	LocaleChinese    Locale = "zh"
)

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
func (c *Client) PostMyMetricsInBatch(payload []MetricsPayload) error {
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
		Cursor string `json:"cursor"`
		Data   []Vote `json:"data"`
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
