// Package instagram publishes images, carousels and Reels through Meta's
// Graph API.
package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the Instagram publishing API.
type Client struct {
	Token  string
	UserID string
	Base   string
	HTTP   *http.Client
	Poll   time.Duration // wait between status checks
	Log    func(format string, args ...any)
}

// New builds a client. Tokens from Meta's "Unified login" start with EAA
// and are served by graph.facebook.com; older Instagram-login tokens use
// graph.instagram.com. IG_API_BASE overrides both.
func New(token, userID string) *Client {
	token = strings.TrimSpace(token)
	base := "https://graph.instagram.com"
	if strings.HasPrefix(token, "EAA") {
		base = "https://graph.facebook.com"
	}
	if v := os.Getenv("IG_API_BASE"); v != "" {
		base = strings.TrimRight(v, "/")
	}
	return &Client{
		Token:  token,
		UserID: strings.TrimSpace(userID),
		Base:   base,
		HTTP:   &http.Client{Timeout: 60 * time.Second},
		Poll:   3 * time.Second,
		Log:    func(string, ...any) {},
	}
}

// APIError is an error returned by the Graph API.
type APIError struct {
	Status    int
	Message   string `json:"message"`
	Code      int    `json:"code"`
	Transient bool   `json:"is_transient"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Instagram API error (HTTP %d, code %d): %s", e.Status, e.Code, e.Message)
}

// call makes one request and decodes the JSON reply. Transient failures are
// retried with a growing pause.
func (c *Client) call(method, path string, params url.Values) (map[string]any, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * c.Poll)
		}
		out, err := c.once(method, path, params)
		if err == nil {
			return out, nil
		}
		last = err
		var api *APIError
		if errors.As(err, &api) && !api.Transient && api.Status < 500 {
			return nil, err // a real refusal: retrying will not help
		}
	}
	return nil, last
}

func (c *Client) once(method, path string, params url.Values) (map[string]any, error) {
	q := url.Values{}
	for k, v := range params {
		q[k] = v
	}
	q.Set("access_token", c.Token)
	u := c.Base + "/" + path
	var body io.Reader
	if method == http.MethodGet {
		u += "?" + q.Encode()
	} else {
		body = strings.NewReader(q.Encode())
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s /%s: network error", method, path) // never echo the URL: it holds the token
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if e, ok := out["error"]; ok || resp.StatusCode >= 400 {
		api := &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		if ok {
			if b, err := json.Marshal(e); err == nil {
				_ = json.Unmarshal(b, api)
			}
		}
		if len(api.Message) > 400 {
			api.Message = api.Message[:400]
		}
		return nil, api
	}
	return out, nil
}

func str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	}
	return ""
}

// User returns the Instagram account ID, looking it up when not configured.
func (c *Client) User() (string, error) {
	if c.UserID != "" {
		return c.UserID, nil
	}
	if !strings.HasPrefix(c.Token, "EAA") {
		out, err := c.call(http.MethodGet, "me", url.Values{"fields": {"user_id"}})
		if err != nil {
			return "", err
		}
		c.UserID = str(out, "user_id")
		return c.UserID, nil
	}
	out, err := c.call(http.MethodGet, "me/accounts", url.Values{"fields": {"instagram_business_account"}})
	if err == nil {
		if pages, ok := out["data"].([]any); ok {
			for _, p := range pages {
				if page, ok := p.(map[string]any); ok {
					if acct, ok := page["instagram_business_account"].(map[string]any); ok {
						c.UserID = str(acct, "id")
						return c.UserID, nil
					}
				}
			}
		}
	}
	return "", errors.New("could not work out the Instagram account ID from this token; " +
		"add a repository secret named IG_USER_ID with the ID shown next to your account on the Meta developer dashboard")
}

// container creates a media container. optional parameters are niceties
// (alt text, a cover image): if Instagram rejects the request with them, it
// is retried once without.
func (c *Client) container(required, optional url.Values) (string, error) {
	user, err := c.User()
	if err != nil {
		return "", err
	}
	params := url.Values{}
	for k, v := range required {
		params[k] = v
	}
	for k, v := range optional {
		if len(v) > 0 && v[0] != "" {
			params[k] = v
		}
	}
	out, err := c.call(http.MethodPost, user+"/media", params)
	if err != nil && len(params) > len(required) {
		c.Log("instagram: retrying without optional fields (%v)", err)
		out, err = c.call(http.MethodPost, user+"/media", required)
	}
	if err != nil {
		return "", err
	}
	return str(out, "id"), nil
}

// wait blocks until Instagram has downloaded and processed a container.
func (c *Client) wait(id string, tries int) error {
	for i := 0; i < tries; i++ {
		out, err := c.call(http.MethodGet, id, url.Values{"fields": {"status_code,status"}})
		if err != nil {
			return err
		}
		switch str(out, "status_code") {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("Instagram could not process the upload: %s %s", str(out, "status_code"), str(out, "status"))
		}
		time.Sleep(c.Poll)
	}
	return errors.New("Instagram took too long to process the upload")
}

func (c *Client) publish(container string) (string, error) {
	user, err := c.User()
	if err != nil {
		return "", err
	}
	out, err := c.call(http.MethodPost, user+"/media_publish", url.Values{"creation_id": {container}})
	if err != nil {
		return "", err
	}
	return str(out, "id"), nil
}

// PublishImage posts a single image and returns the media ID.
func (c *Client) PublishImage(imageURL, caption, alt string) (string, error) {
	id, err := c.container(url.Values{"image_url": {imageURL}, "caption": {caption}}, url.Values{"alt_text": {alt}})
	if err != nil {
		return "", err
	}
	if err := c.wait(id, 40); err != nil {
		return "", err
	}
	return c.publish(id)
}

// PublishCarousel posts several images as one swipeable post.
func (c *Client) PublishCarousel(imageURLs []string, caption string, alts []string) (string, error) {
	var children []string
	for i, u := range imageURLs {
		alt := ""
		if i < len(alts) {
			alt = alts[i]
		}
		id, err := c.container(url.Values{"image_url": {u}, "is_carousel_item": {"true"}}, url.Values{"alt_text": {alt}})
		if err != nil {
			return "", fmt.Errorf("slide %d: %w", i+1, err)
		}
		if err := c.wait(id, 40); err != nil {
			return "", fmt.Errorf("slide %d: %w", i+1, err)
		}
		children = append(children, id)
	}
	id, err := c.container(url.Values{"media_type": {"CAROUSEL"}, "children": {strings.Join(children, ",")}, "caption": {caption}}, nil)
	if err != nil {
		return "", err
	}
	if err := c.wait(id, 40); err != nil {
		return "", err
	}
	return c.publish(id)
}

// PublishReel posts a video as a Reel that also appears in the feed.
func (c *Client) PublishReel(videoURL, coverURL, caption string) (string, error) {
	id, err := c.container(
		url.Values{"media_type": {"REELS"}, "video_url": {videoURL}, "caption": {caption}, "share_to_feed": {"true"}},
		url.Values{"cover_url": {coverURL}})
	if err != nil {
		return "", err
	}
	if err := c.wait(id, 120); err != nil { // video processing can take a few minutes
		return "", err
	}
	return c.publish(id)
}
