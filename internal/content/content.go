// Package content holds the post queue: its types, loading, validation,
// selection and captions.
package content

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Slide is one page of a carousel. It is either a text slide (Kicker, Head,
// Body) or a quotation (Quote, By).
type Slide struct {
	Kicker string `json:"k,omitempty"`
	Head   string `json:"h,omitempty"`
	Body   string `json:"b,omitempty"`
	Quote  string `json:"q,omitempty"`
	By     string `json:"by,omitempty"`
}

// IsQuote reports whether the slide is a quotation.
func (s Slide) IsQuote() bool { return s.Quote != "" }

// ImageRef points an art post at a museum record.
type ImageRef struct {
	AIC  string `json:"aic,omitempty"`
	Wiki string `json:"wiki,omitempty"`
}

// Post is one entry in posts.json.
type Post struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Label      string    `json:"label"`
	Title      string    `json:"title"`
	Year       int       `json:"year,omitempty"`
	FirstAired int       `json:"first_aired,omitempty"`
	Director   string    `json:"director,omitempty"`
	Author     string    `json:"author,omitempty"`
	Artist     string    `json:"artist,omitempty"`
	Creator    string    `json:"creator,omitempty"`
	Episode    []int     `json:"episode,omitempty"`
	Hook       string    `json:"hook,omitempty"`
	Quote      string    `json:"quote,omitempty"`
	Slides     []Slide   `json:"slides"`
	Caption    string    `json:"caption"`
	Tags       string    `json:"tags"`
	Image      *ImageRef `json:"image,omitempty"`
	Detail     []float64 `json:"detail,omitempty"`
	Wiki       string    `json:"wiki,omitempty"`
	CoverTitle string    `json:"cover_title,omitempty"`
	SkipStills int       `json:"skip_stills,omitempty"`
	// Ask replaces the series' usual closing line with one written for this post.
	Ask string `json:"ask,omitempty"`
}

// Maker returns whoever made the work: director, author, artist or creator.
func (p Post) Maker() string {
	for _, s := range []string{p.Director, p.Author, p.Artist, p.Creator} {
		if s != "" {
			return s
		}
	}
	return ""
}

// SearchTitle is the title used when looking for a cover image.
func (p Post) SearchTitle() string {
	if p.CoverTitle != "" {
		return p.CoverTitle
	}
	return p.Title
}

// Outro is the line on the closing slide: a prompt to send, save or reply,
// which are the actions Instagram weighs most. Single-image posts have none.
func (p Post) Outro(cfg Config) string {
	if p.Quote != "" || len(p.Slides) == 0 {
		return ""
	}
	if p.Ask != "" {
		return p.Ask
	}
	return cfg.Outro[p.Label]
}

// SlideCount is the number of images the post produces.
func (p Post) SlideCount(cfg Config) int {
	if p.Quote != "" {
		return 1
	}
	n := 1 + len(p.Slides)
	if p.Outro(cfg) != "" {
		n++
	}
	return n
}

// Quote is one entry in quotes.json, the fallback used when the queue is empty.
type Quote struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Source string `json:"source"`
	By     string `json:"by"`
}

// Config is config.json.
type Config struct {
	Handle               string            `json:"handle"`
	SwipeHint            string            `json:"swipe_hint"`
	Hashtags             map[string]string `json:"hashtags"`
	MaxHashtags          int               `json:"max_hashtags"`
	CaptionFooter        string            `json:"caption_footer"`
	Outro                map[string]string `json:"outro"` // closing line per series label
	ImageCreditInCaption bool              `json:"image_credit_in_caption"`
}

// Entry records one published item in posted.json.
type Entry struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Date string `json:"date"`
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// LoadPosts reads posts.json.
func LoadPosts(path string) ([]Post, error) {
	var posts []Post
	return posts, readJSON(path, &posts)
}

// LoadQuotes reads quotes.json. A missing file is not an error.
func LoadQuotes(path string) ([]Quote, error) {
	var quotes []Quote
	err := readJSON(path, &quotes)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return quotes, err
}

// LoadConfig reads config.json and fills in defaults.
func LoadConfig(path string) (Config, error) {
	cfg := Config{MaxHashtags: 5}
	if err := readJSON(path, &cfg); err != nil {
		return cfg, err
	}
	if cfg.MaxHashtags <= 0 {
		cfg.MaxHashtags = 5
	}
	return cfg, nil
}

// LoadPosted reads posted.json. A missing file means nothing has been posted.
func LoadPosted(path string) ([]Entry, error) {
	var entries []Entry
	err := readJSON(path, &entries)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return entries, err
}

// SavePosted writes posted.json.
func SavePosted(path string, entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// QuoteID is a stable identifier for a fallback quote.
func QuoteID(q Quote) string {
	id := strings.ToLower(q.Source + "-" + q.Text)
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
		if b.Len() >= 48 {
			break
		}
	}
	return "quote-" + strings.Trim(b.String(), "-")
}
