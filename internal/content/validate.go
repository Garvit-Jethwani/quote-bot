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
	"RANDOM THOUGHT": {[]string{"thought"}, 0, 0},
	"DEADPAN":        {[]string{"bit"}, 3, 6},
	"ROAST":          {[]string{"bit"}, 3, 6},
	"SHORT STORY":    {[]string{"bit"}, 3, 6},
}

const (
	maxBodyWords = 62
	maxPageWords = 48
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
		if p.Doodle != "" && !Doodles[p.Doodle] {
			add(p, "unknown doodle %q", p.Doodle)
		}
		if n := len(p.Slides); n < series.Min || n > series.Max {
			add(p, "%s needs %d to %d slides, has %d", p.Label, series.Min, series.Max, n)
		}

		quoteCard := p.Kind == "comedy" || p.Kind == "take" || p.Kind == "thought"
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
			if p.Year == 0 && p.Kind != "bit" {
				add(p, "missing year")
			}
			if p.Maker() == "" && p.Kind != "bit" {
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
			case s.Text != "":
				if w := len(strings.Fields(s.Text)); w > maxPageWords {
					add(p, "slide %d: page is %d words; keep it to %d", n+1, w, maxPageWords)
				}
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
	out := []string{p.Hook, p.Quote, p.Caption, p.Title, p.Tags, p.Ask}
	for _, s := range p.Slides {
		out = append(out, s.Kicker, s.Head, s.Body, s.Quote, s.By, s.Text)
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

// Doodles are the drawings a post can name in its "doodle" field. The render
// package draws them; its tests check the two lists agree.
var Doodles = map[string]bool{
	"bat": true, "bone": true, "book": true, "bulb": true, "clapper": true, "clock": true,
	"dumbbell": true, "flame": true, "fridge": true, "ghost": true, "lift": true, "martini": true,
	"mic": true, "mug": true, "parrot": true, "phone": true, "plate": true, "ship": true,
	"van": true, "wifi": true, "wizard": true,
}
