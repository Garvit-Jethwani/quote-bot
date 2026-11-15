package instagram

import (
	"net/http"
	"net/url"
)

// Account is the profile the token posts to.
type Account struct {
	Username  string
	Followers int
	Posts     int
}

// Media is one published post with its public counters.
type Media struct {
	ID        string
	Likes     int
	Comments  int
	Type      string
	Permalink string
	Timestamp string
}

func num(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// Account returns the profile's name and counts.
func (c *Client) Account() (Account, error) {
	user, err := c.User()
	if err != nil {
		return Account{}, err
	}
	out, err := c.call(http.MethodGet, user, url.Values{"fields": {"username,followers_count,media_count"}})
	if err != nil {
		return Account{}, err
	}
	return Account{Username: str(out, "username"), Followers: num(out, "followers_count"), Posts: num(out, "media_count")}, nil
}

// Quota reports how many API-published posts were used in the last 24 hours.
func (c *Client) Quota() (used int, err error) {
	user, err := c.User()
	if err != nil {
		return 0, err
	}
	out, err := c.call(http.MethodGet, user+"/content_publishing_limit", url.Values{"fields": {"quota_usage"}})
	if err != nil {
		return 0, err
	}
	if data, ok := out["data"].([]any); ok && len(data) > 0 {
		if row, ok := data[0].(map[string]any); ok {
			return num(row, "quota_usage"), nil
		}
	}
	return 0, nil
}

// RecentMedia returns up to max of the account's latest posts, newest first.
func (c *Client) RecentMedia(max int) ([]Media, error) {
	user, err := c.User()
	if err != nil {
		return nil, err
	}
	var all []Media
	params := url.Values{"fields": {"id,like_count,comments_count,media_type,permalink,timestamp"}, "limit": {"50"}}
	for len(all) < max {
		out, err := c.call(http.MethodGet, user+"/media", params)
		if err != nil {
			return all, err
		}
		rows, _ := out["data"].([]any)
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				all = append(all, Media{ID: str(m, "id"), Likes: num(m, "like_count"), Comments: num(m, "comments_count"),
					Type: str(m, "media_type"), Permalink: str(m, "permalink"), Timestamp: str(m, "timestamp")})
			}
		}
		after := ""
		if paging, ok := out["paging"].(map[string]any); ok {
			if cursors, ok := paging["cursors"].(map[string]any); ok {
				after = str(cursors, "after")
			}
			if _, more := paging["next"]; !more {
				after = ""
			}
		}
		if after == "" || len(rows) == 0 {
			break
		}
		params.Set("after", after)
	}
	if len(all) > max {
		all = all[:max]
	}
	return all, nil
}
