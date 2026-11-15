package content

import (
	"math/rand"
	"strings"
	"testing"
)

func repoPosts(t *testing.T) []Post {
	t.Helper()
	posts, err := LoadPosts("../../posts.json")
	if err != nil {
		t.Fatal(err)
	}
	return posts
}

func TestRepositoryQueueIsValid(t *testing.T) {
	if problems := Validate(repoPosts(t)); len(problems) > 0 {
		t.Fatalf("posts.json has problems:\n%s", strings.Join(problems, "\n"))
	}
}

func TestValidateCatchesMistakes(t *testing.T) {
	good := Post{ID: "a", Kind: "film", Label: "SCENE STUDY", Title: "T", Year: 2000, Director: "D", Hook: "A hook",
		Caption: "c", Tags: "#a", Slides: []Slide{{"K", "H", "B", "", ""}, {"K", "H", "B", "", ""}, {"K", "H", "B", "", ""}}}
	if p := Validate([]Post{good}); len(p) != 0 {
		t.Fatalf("valid post rejected: %v", p)
	}
	cases := map[string]func(*Post){
		"duplicate id":     nil,
		"unknown label":    func(p *Post) { p.Label = "NOPE" },
		"cannot have kind": func(p *Post) { p.Kind = "book" },
		"missing caption":  func(p *Post) { p.Caption = " " },
		"tags":             func(p *Post) { p.Tags = "#a #b #c #d #e" },
		"slides":           func(p *Post) { p.Slides = p.Slides[:1] },
		"missing hook":     func(p *Post) { p.Hook = "" },
		"missing year":     func(p *Post) { p.Year = 0 },
		"dash or arrow":    func(p *Post) { p.Caption = "one — two" },
		"needs k, h and b": func(p *Post) { p.Slides[0].Body = "" },
		"body is":          func(p *Post) { p.Slides[0].Body = strings.Repeat("word ", 80) },
		"has no source":    func(p *Post) { p.Slides[0] = Slide{Quote: "q"} },
		"missing director": func(p *Post) { p.Director = "" },
	}
	for want, mutate := range cases {
		p := good
		p.Slides = append([]Slide(nil), good.Slides...)
		posts := []Post{p}
		if mutate == nil {
			posts = []Post{p, p}
		} else {
			mutate(&posts[0])
		}
		got := strings.Join(Validate(posts), "\n")
		if !strings.Contains(got, want) {
			t.Errorf("expected a problem containing %q, got %q", want, got)
		}
	}
}

func TestNextNeverRepeatsAndVariesSeries(t *testing.T) {
	posts := repoPosts(t)
	rng := rand.New(rand.NewSource(1))
	var posted []Entry
	seen := map[string]bool{}
	byID := map[string]Post{}
	for _, p := range posts {
		byID[p.ID] = p
	}
	prev := ""
	sameRunning := 0
	for {
		p, ok := Next(posts, posted, "", false, rng)
		if !ok {
			break
		}
		if seen[p.ID] {
			t.Fatalf("%s picked twice", p.ID)
		}
		seen[p.ID] = true
		if p.Label == prev {
			sameRunning++
		}
		prev = p.Label
		posted = append(posted, Entry{ID: p.ID, Type: "post"})
	}
	if len(seen) != len(posts) {
		t.Fatalf("picked %d of %d posts", len(seen), len(posts))
	}
	// Repeats are only allowed once the other series have run out.
	if sameRunning > len(posts)/3 {
		t.Errorf("same series twice running %d times out of %d", sameRunning, len(posts))
	}
}

func TestALargeSeriesDoesNotDominate(t *testing.T) {
	var posts []Post
	for i := 0; i < 90; i++ {
		posts = append(posts, Post{ID: "joke" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Label: "ONE-LINER"})
	}
	for _, l := range []string{"SCENE STUDY", "WHAT IF", "LINER NOTES"} {
		for i := 0; i < 10; i++ {
			posts = append(posts, Post{ID: l + string(rune('a'+i)), Label: l})
		}
	}
	rng := rand.New(rand.NewSource(7))
	jokes := 0
	for i := 0; i < 400; i++ {
		if p, _ := Next(posts, nil, "", false, rng); p.Label == "ONE-LINER" {
			jokes++
		}
	}
	// 75% of the queue is jokes, but they should come up about one time in four.
	if jokes > 140 {
		t.Errorf("the largest series was picked %d times out of 400", jokes)
	}
}

func TestNextOptions(t *testing.T) {
	posts := []Post{
		{ID: "card", Label: "ONE-LINER"},
		{ID: "deck", Label: "SCENE STUDY", Slides: make([]Slide, 3)},
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		if p, _ := Next(posts, nil, "", true, rng); p.ID != "deck" {
			t.Fatalf("a Reel must use a multi-slide post, got %s", p.ID)
		}
	}
	if p, ok := Next(posts, nil, "card", false, rng); !ok || p.ID != "card" {
		t.Error("asking for a post by id should return it")
	}
	if _, ok := Next(posts, nil, "missing", false, rng); ok {
		t.Error("an unknown id should not be found")
	}
	if _, ok := Next(posts, []Entry{{ID: "card"}, {ID: "deck"}}, "", false, rng); ok {
		t.Error("nothing should be left once everything is posted")
	}
}

func TestHashtags(t *testing.T) {
	got := Hashtags(5, "#one #two two", "#ONE #three #four #five #six")
	if got != "#one #two #three #four #five" {
		t.Errorf("got %q", got)
	}
}

func TestCaptionsFollowTheHouseRules(t *testing.T) {
	cfg, err := LoadConfig("../../config.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range repoPosts(t) {
		c := Caption(p, cfg, "Stills via TMDB.")
		if strings.ContainsAny(c, "—–→") {
			t.Errorf("%s: caption contains a dash or arrow", p.ID)
		}
		if n := strings.Count(c, "#"); n < 1 || n > cfg.MaxHashtags {
			t.Errorf("%s: %d hashtags", p.ID, n)
		}
		if strings.Contains(c, "TMDB") && !cfg.ImageCreditInCaption {
			t.Errorf("%s: image credit leaked into the caption", p.ID)
		}
		if len([]rune(c)) > maxCaption {
			t.Errorf("%s: caption too long", p.ID)
		}
		for i := 0; i < p.SlideCount(cfg); i++ {
			if AltText(p, i) == "" {
				t.Errorf("%s: slide %d has no alt text", p.ID, i+1)
			}
		}
	}
}

func TestNextQuoteAlternates(t *testing.T) {
	quotes := []Quote{{Type: "book", Text: "a", Source: "A"}, {Type: "movie", Text: "b", Source: "B"}}
	rng := rand.New(rand.NewSource(1))
	q, _ := NextQuote(quotes, nil, rng)
	if q.Type != "book" {
		t.Errorf("first fallback should be a book quote, got %s", q.Type)
	}
	q, _ = NextQuote(quotes, []Entry{{ID: QuoteID(quotes[0]), Type: "book"}}, rng)
	if q.Type != "movie" {
		t.Errorf("after a book quote comes a movie quote, got %s", q.Type)
	}
}
