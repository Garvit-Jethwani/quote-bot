package content

import (
	"fmt"
	"strings"
)

// Series maps each label to the kinds it may use and its slide-count range.
var Series = map[string]struct {
	Kinds    []string
	Min, Max int
}{
	"SCENE STUDY":    {[]string{"film"}, 3, 5},
	"WHAT IF":        {[]string{"film", "series"}, 3, 4},
	"EPISODE STUDY":  {[]string{"series"}, 3, 4},
	"DID YOU NOTICE": {[]string{"film", "series"}, 3, 5},
	"ONE FRAME":      {[]string{"film", "series"}, 0, 0},
	"LOOK CLOSER":    {[]string{"art"}, 3, 5},
	"SHELF NOTES":    {[]string{"book"}, 3, 5},
	"LINER NOTES":    {[]string{"music"}, 3, 5},
	"ONE-LINER":      {[]string{"comedy"}, 0, 0},
	"HOT TAKE":       {[]string{"take"}, 0, 0},
}

const (
	maxBodyWords = 62
	maxHookWords = 16
	maxTags      = 4
)

// Validate checks every post and returns one message per problem. An empty
// result means the queue is safe to publish from.
func Validate(posts []Post) []string {
	var problems []string
	add := func(p Post, format string, args ...any) {
		problems = append(problems, p.ID+": "+fmt.Sprintf(format, args...))
	}
	seen := map[string]bool{}
	for i, p := range posts {
		if p.ID == "" {
			problems = append(problems, fmt.Sprintf("post %d has no id", i+1))
			continue
		}
		if seen[p.ID] {
			add(p, "duplicate id")
		}
		seen[p.ID] = true

		series, ok := Series[p.Label]
		if !ok {
			add(p, "unknown label %q", p.Label)
			continue
		}
		if !contains(series.Kinds, p.Kind) {
			add(p, "label %q cannot have kind %q", p.Label, p.Kind)
		}
		if p.Title == "" {
			add(p, "missing title")
		}
		if strings.TrimSpace(p.Caption) == "" {
			add(p, "missing caption")
		}
		if n := len(strings.Fields(p.Tags)); n == 0 || n > maxTags {
			add(p, "needs 1 to %d tags, has %d", maxTags, n)
		}
		if n := len(p.Slides); n < series.Min || n > series.Max {
			add(p, "%s needs %d to %d slides, has %d", p.Label, series.Min, series.Max, n)
		}

		quoteCard := p.Kind == "comedy" || p.Kind == "take"
		if quoteCard {
			if p.Quote == "" {
				add(p, "missing quote")
			}
		} else {
			if p.Hook == "" {
				add(p, "missing hook")
			}
			if n := len(strings.Fields(p.Hook)); n > maxHookWords {
				add(p, "hook is %d words; keep it to %d", n, maxHookWords)
			}
			if p.Year == 0 {
				add(p, "missing year")
			}
			if p.Maker() == "" {
				add(p, "missing director, author, artist or creator")
			}
		}
		if p.Kind == "art" && (p.Image == nil || p.Image.AIC == "") {
			add(p, "art posts need image.aic")
		}
		if len(p.Episode) != 0 && len(p.Episode) != 2 {
			add(p, "episode must be [season, number]")
		}
		if len(p.Detail) != 0 && len(p.Detail) != 2 {
			add(p, "detail must be [x, y]")
		}

		for n, s := range p.Slides {
			switch {
			case s.IsQuote():
				if s.By == "" {
					add(p, "slide %d: quotation has no source", n+1)
				}
			case s.Kicker == "" || s.Head == "" || s.Body == "":
				add(p, "slide %d: needs k, h and b", n+1)
			default:
				if w := len(strings.Fields(s.Body)); w > maxBodyWords {
					add(p, "slide %d: body is %d words; keep it to %d", n+1, w, maxBodyWords)
				}
			}
		}
		for _, text := range p.texts() {
			if strings.ContainsAny(text, "—–→") {
				add(p, "contains a dash or arrow: %q", clipTo(text, 50))
				break
			}
		}
	}
	return problems
}

func (p Post) texts() []string {
	out := []string{p.Hook, p.Quote, p.Caption, p.Title, p.Tags}
	for _, s := range p.Slides {
		out = append(out, s.Kicker, s.Head, s.Body, s.Quote, s.By)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func clipTo(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}
