package dbl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// https://docs.top.gg/api/v0/bots#get-/bots
type BotQueryOptions struct {
	Limit  int
	Offset int
	Search string
	Sort   string
	Fields string
}

// https://docs.top.gg/api/v0/bots#get-/bots
type BotQueryResponse struct {
	Results []Bot `json:"results"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	Count   int   `json:"count"`
	Total   int   `json:"total"`
}

// https://docs.top.gg/api/v0/bots#bot-structure
type Bot struct {
	ID               Snowflake   `json:"id"`
	Username         string      `json:"username"`
	Discriminator    string      `json:"discriminator"`
	Avatar           string      `json:"avatar,omitempty"`
	DefAvatar        string      `json:"defAvatar,omitempty"`
	Library          string      `json:"lib"`
	Prefix           string      `json:"prefix"`
	ShortDescription string      `json:"shortdesc"`
	LongDescription  string      `json:"longdesc,omitempty"`
	Tags             []string    `json:"tags,omitzero"`
	Website          string      `json:"website,omitempty"`
	Support          string      `json:"support,omitempty"`
	Github           string      `json:"github,omitempty"`
	Owners           []Snowflake `json:"owners,omitzero"`
	GuildAffiliation []Snowflake `json:"guilds,omitzero"`
	Invite           string      `json:"invite,omitempty"`
	Date             time.Time   `json:"date"`
	ServerCount      int         `json:"server_count,omitempty"`
	ShardCount       int         `json:"shard_count,omitempty"`
	CertifiedBot     bool        `json:"certifiedBot"`
	Vanity           string      `json:"vanity,omitempty"`
	Points           int         `json:"points"`
	MonthlyPoints    int         `json:"monthlyPoints"`
	DonateBotGuildID string      `json:"donatebotguildid"`
}

// https://docs.top.gg/api/v0/bots#get-/bots/bot_id/stats
// https://docs.top.gg/api/v0/bots#post-/bots/bot_id/stats
type BotStats struct {
	ServerCount int   `json:"server_count,omitempty"`
	Shards      []int `json:"shards,omitzero"`
	ShardCount  int   `json:"shard_count,omitempty"`
}

// https://docs.top.gg/api/v0/bots#get-/bots/bot_id/check
type botVotedResponse struct {
	Voted int `json:"voted"`
}

// https://docs.top.gg/api/v0/bots#get-/bots
func (c *Client) GetBots(options *BotQueryOptions) (*BotQueryResponse, error) {
	q := url.Values{}
	if options != nil {
		if options.Limit > 0 {
			q.Set("limit", strconv.Itoa(options.Limit))
		}
		if options.Offset > 0 {
			q.Set("offset", strconv.Itoa(options.Offset))
		}
		if options.Search != "" {
			q.Set("search", options.Search)
		}
		if options.Sort != "" {
			q.Set("sort", options.Sort)
		}
		if options.Fields != "" {
			q.Set("fields", options.Fields)
		}
	}

	route := "/v0/bots"
	if len(q) > 0 {
		route += "?" + q.Encode()
	}

	b, err := c.request(http.MethodGet, route, nil)
	if err != nil {
		return nil, err
	}

	var res BotQueryResponse
	err = json.Unmarshal(b, &res)
	return &res, err
}

// https://docs.top.gg/api/v0/bots#get-/bots/bot_id/stats
func (c *Client) GetBotStats(botID Snowflake) (*BotStats, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v0/bots/%s/stats", botID), nil)
	if err != nil {
		return nil, err
	}

	var stats BotStats
	err = json.Unmarshal(b, &stats)
	return &stats, err
}

// https://docs.top.gg/api/v0/bots#get-/bots
func (c *Client) GetBot(id Snowflake) (*Bot, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v0/bots/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var bot Bot
	err = json.Unmarshal(b, &bot)
	return &bot, err
}

// Fetches last 1000 votes of a specific bot.
// https://docs.top.gg/api/v0/bots#get-%2Fbots%2Fbot_id%2Fvotes
func (c *Client) GetBotVotes(botID Snowflake) ([]User, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v0/bots/%s/votes", botID), nil)
	if err != nil {
		return nil, err
	}

	var users []User
	err = json.Unmarshal(b, &users)
	return users, err
}

// Checks if a user has voted for a specific bot.
// https://docs.top.gg/api/v0/bots#get-/bots/bot_id/check
func (c *Client) GetBotVote(botID, userID Snowflake) (bool, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v0/bots/%s/check?userId=%s", botID, userID), nil)
	if err != nil {
		return false, err
	}

	var res botVotedResponse
	err = json.Unmarshal(b, &res)
	if err != nil {
		return false, err
	}

	return res.Voted == 1, nil
}

// https://docs.top.gg/api/v0/bots#post-/bots/bot_id/stats
func (c *Client) PostBotStats(botID Snowflake, stats BotStats) error {
	_, err := c.request(http.MethodPost, fmt.Sprintf("/v0/bots/%s/stats", botID), stats)
	return err
}
