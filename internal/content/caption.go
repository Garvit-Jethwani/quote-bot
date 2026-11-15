package content

import (
	"fmt"
	"strings"
)

const maxCaption = 2200 // Instagram's caption limit, in characters

// Hashtags merges groups of hashtags, specific ones first, drops repeats and
// caps the total.
func Hashtags(max int, groups ...string) string {
	seen := map[string]bool{}
	var out []string
	for _, tag := range strings.Fields(strings.Join(groups, " ")) {
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return strings.Join(out, " ")
}

// Credit is the plain-words line naming the work, which Instagram search
// reads. Books, records and jokes carry it in the caption text instead.
func Credit(p Post) string {
	switch p.Kind {
	case "film":
		return fmt.Sprintf("%s (%d), dir. %s.", p.Title, p.Year, p.Director)
	case "series":
		if len(p.Episode) == 2 {
			return fmt.Sprintf("%s, season %d episode %d.", p.Title, p.Episode[0], p.Episode[1])
		}
		return fmt.Sprintf("%s (%d).", p.Title, p.Year)
	case "art":
		return fmt.Sprintf("%s, %s (%d).", p.Artist, p.Title, p.Year)
	case "music":
		return fmt.Sprintf("%s by %s (%d).", p.Title, p.Artist, p.Year)
	case "book":
		return fmt.Sprintf("%s by %s.", p.Title, p.Author)
	}
	return ""
}

// Caption builds the text under a post.
func Caption(p Post, cfg Config, imageCredit string) string {
	credit := Credit(p)
	if cfg.ImageCreditInCaption && imageCredit != "" {
		credit = strings.TrimSpace(credit + " " + imageCredit)
	}
	text := p.Caption
	if p.Kind == "take" || p.Kind == "thought" { // say what it is before anything else
		text = fmt.Sprintf("%s no. %d.\n\n%s", strings.ToLower(p.Label), p.Number, text)
	}
	parts := []string{text, credit, cfg.CaptionFooter,
		Hashtags(cfg.MaxHashtags, p.Tags, cfg.Hashtags[p.Kind])}
	return join(parts)
}

// QuoteCaption builds the text under a fallback quote card.
func QuoteCaption(q Quote, cfg Config) string {
	text := q.Text
	if !strings.HasPrefix(text, "“") {
		text = "“" + text + "”"
	}
	credit := fmt.Sprintf("%s, %s", q.By, q.Source)
	key := "book_quote"
	if q.Type == "movie" {
		credit = fmt.Sprintf("%s (%s)", q.Source, q.By)
		key = "movie"
	}
	return join([]string{text, credit, cfg.CaptionFooter, Hashtags(cfg.MaxHashtags, cfg.Hashtags[key])})
}

func join(parts []string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	out := []rune(strings.Join(kept, "\n\n"))
	if len(out) > maxCaption {
		out = out[:maxCaption]
	}
	return string(out)
}

// AltText describes the post for screen readers and search.
func AltText(p Post, slide int) string {
	if p.Quote != "" {
		if p.Kind == "comedy" {
			return clip(fmt.Sprintf("Quote by %s: %s", p.Title, p.Quote))
		}
		return clip(p.Quote)
	}
	if slide == 0 {
		return clip(strings.TrimSpace(fmt.Sprintf("%s %s %s", p.Hook, p.Title, Credit(p))))
	}
	if slide > 0 && slide-1 < len(p.Slides) {
		s := p.Slides[slide-1]
		if s.Text != "" {
			return clip(s.Text)
		}
		if s.IsQuote() {
			return clip(s.Quote + " " + s.By)
		}
		return clip(s.Head + ". " + s.Body)
	}
	return clip(p.Title)
}

func clip(s string) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > 950 {
		r = r[:950]
	}
	return string(r)
}
