package content

import "math/rand"

// Next picks the post to publish. With wanted set it returns that post.
// Otherwise it picks a series at random, then an unposted post within it,
// avoiding the series (label) of the last two published posts so the feed
// never repeats itself and no series crowds out the others.
// multiSlide restricts the choice to posts with at least two text slides,
// which is what a Reel needs. It returns false when nothing is left.
func Next(posts []Post, posted []Entry, wanted string, multiSlide bool, rng *rand.Rand) (Post, bool) {
	if wanted != "" {
		for _, p := range posts {
			if p.ID == wanted {
				return p, true
			}
		}
		return Post{}, false
	}
	used := make(map[string]bool, len(posted))
	for _, e := range posted {
		used[e.ID] = true
	}
	byID := make(map[string]Post, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	recent := map[string]bool{}
	for i := len(posted) - 1; i >= 0 && i >= len(posted)-2; i-- {
		if p, ok := byID[posted[i].ID]; ok {
			recent[p.Label] = true
		}
	}
	var fresh, varied []Post
	for _, p := range posts {
		if used[p.ID] || (multiSlide && len(p.Slides) < 2) {
			continue
		}
		fresh = append(fresh, p)
		if !recent[p.Label] {
			varied = append(varied, p)
		}
	}
	if len(fresh) == 0 {
		return Post{}, false
	}
	if len(varied) == 0 {
		varied = fresh
	}
	// Choose a series first, then a post within it. Picking a post directly
	// would let whichever series has the most entries dominate the feed.
	bySeries := map[string][]Post{}
	var labels []string
	for _, p := range varied {
		if _, ok := bySeries[p.Label]; !ok {
			labels = append(labels, p.Label)
		}
		bySeries[p.Label] = append(bySeries[p.Label], p)
	}
	pool := bySeries[labels[rng.Intn(len(labels))]]
	return pool[rng.Intn(len(pool))], true
}

// Remaining counts unposted posts.
func Remaining(posts []Post, posted []Entry) int {
	used := make(map[string]bool, len(posted))
	for _, e := range posted {
		used[e.ID] = true
	}
	n := 0
	for _, p := range posts {
		if !used[p.ID] {
			n++
		}
	}
	return n
}

// NextQuote picks a fallback quote, alternating book and movie quotes and
// preferring ones that have not been used.
func NextQuote(quotes []Quote, posted []Entry, rng *rand.Rand) (Quote, bool) {
	if len(quotes) == 0 {
		return Quote{}, false
	}
	kind := "book"
	for i := len(posted) - 1; i >= 0; i-- {
		if t := posted[i].Type; t == "book" || t == "movie" {
			if t == "book" {
				kind = "movie"
			}
			break
		}
	}
	used := make(map[string]bool, len(posted))
	for _, e := range posted {
		used[e.ID] = true
	}
	var pool, fresh []Quote
	for _, q := range quotes {
		if q.Type != kind {
			continue
		}
		pool = append(pool, q)
		if !used[QuoteID(q)] {
			fresh = append(fresh, q)
		}
	}
	if len(pool) == 0 {
		pool = quotes
	}
	if len(fresh) == 0 {
		fresh = pool
	}
	return fresh[rng.Intn(len(fresh))], true
}
