package topgg

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// https://docs.top.gg/api/v0/users#response-fields
type User struct {
	Social             Social    `json:"social"`
	Banner             string    `json:"banner,omitempty"`
	Discriminator      string    `json:"discriminator"`
	Avatar             string    `json:"avatar,omitempty"`
	DefAvatar          string    `json:"defAvatar"`
	Biography          string    `json:"bio,omitempty"`
	Username           string    `json:"username"`
	Color              string    `json:"color,omitempty"`
	ID                 Snowflake `json:"id"`
	Supporter          bool      `json:"supporter"`
	CertifiedDeveloper bool      `json:"certifiedDev"`
	Moderator          bool      `json:"mod"`
	WebsiteModerator   bool      `json:"webMod"`
	Admin              bool      `json:"admin"`
}

// https://docs.top.gg/api/v0/users#param-social
type Social struct {
	Youtube   string `json:"youtube,omitempty"`
	Reddit    string `json:"reddit,omitempty"`
	Twitter   string `json:"twitter,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	Github    string `json:"github,omitempty"`
}

// https://docs.top.gg/api/v0/users#get-/users/user_id
func (c *Client) GetUser(id Snowflake) (*User, error) {
	b, err := c.request(http.MethodGet, fmt.Sprintf("/v0/users/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var user User
	err = json.Unmarshal(b, &user)
	return &user, err
}
