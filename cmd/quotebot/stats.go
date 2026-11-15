package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/Garvit-Jethwani/quote-bot/internal/images"
	"github.com/Garvit-Jethwani/quote-bot/internal/instagram"
)

// postStat is one published post's public counters.
type postStat struct {
	ID       string `json:"id"`
	Series   string `json:"series"`
	Format   string `json:"format"`
	Date     string `json:"date"`
	Likes    int    `json:"likes"`
	Comments int    `json:"comments"`
	Link     string `json:"link,omitempty"`
}

// seriesStat averages a series. Score counts a comment as three likes,
// because a comment takes more effort and Instagram weighs it more.
type seriesStat struct {
	Series      string  `json:"series"`
	Posts       int     `json:"posts"`
	AvgLikes    float64 `json:"avg_likes"`
	AvgComments float64 `json:"avg_comments"`
	Score       float64 `json:"score"`
}

type statsFile struct {
	Updated   string       `json:"updated"`
	Followers int          `json:"followers"`
	Note      string       `json:"note"`
	Series    []seriesStat `json:"series"`
	Posts     []postStat   `json:"posts"`
}

func client() (*instagram.Client, error) {
	token := os.Getenv("IG_TOKEN")
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("IG_TOKEN is not set; add it as a repository secret")
	}
	ig := instagram.New(token, os.Getenv("IG_USER_ID"))
	ig.Log = logf
	if os.Getenv("IG_API_BASE") != "" {
		ig.Poll = 50 * time.Millisecond
	}
	return ig, nil
}

// cmdStats writes stats.json: likes and comments for every post the bot has
// published, and an average per series. The weekly writing task reads it to
// decide what to write more of.
func cmdStats(_ []string) error {
	q, err := load()
	if err != nil {
		return err
	}
	ig, err := client()
	if err != nil {
		return err
	}
	media, err := ig.RecentMedia(500)
	if err != nil {
		return err
	}
	byMedia := map[string]instagram.Media{}
	for _, m := range media {
		byMedia[m.ID] = m
	}
	label := map[string]string{}
	for _, p := range q.posts {
		label[p.ID] = p.Label
	}

	out := statsFile{
		Updated: time.Now().UTC().Format("2006-01-02"),
		Note:    "score = average likes + 3 x average comments. Series with fewer than 3 posts are too new to judge.",
	}
	if acct, err := ig.Account(); err == nil {
		out.Followers = acct.Followers
	}
	sums := map[string]*seriesStat{}
	for _, e := range q.posted {
		m, ok := byMedia[e.Media]
		if !ok || e.Media == "" {
			continue // published before media IDs were recorded, or since deleted
		}
		series := label[e.ID]
		if series == "" {
			series = "QUOTE"
		}
		format := "carousel"
		switch {
		case e.Type == "reel":
			format = "reel"
		case m.Type == "IMAGE":
			format = "single"
		}
		out.Posts = append(out.Posts, postStat{ID: e.ID, Series: series, Format: format, Date: e.Date, Likes: m.Likes, Comments: m.Comments, Link: m.Permalink})
		s := sums[series]
		if s == nil {
			s = &seriesStat{Series: series}
			sums[series] = s
		}
		s.Posts++
		s.AvgLikes += float64(m.Likes)
		s.AvgComments += float64(m.Comments)
	}
	for _, s := range sums {
		s.AvgLikes = round1(s.AvgLikes / float64(s.Posts))
		s.AvgComments = round1(s.AvgComments / float64(s.Posts))
		s.Score = round1(s.AvgLikes + 3*s.AvgComments)
		out.Series = append(out.Series, *s)
	}
	sort.Slice(out.Series, func(i, j int) bool { return out.Series[i].Score > out.Series[j].Score })

	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile("stats.json", append(data, '\n'), 0o644); err != nil {
		return err
	}
	logf("Saved stats for %d post(s) across %d series to stats.json", len(out.Posts), len(out.Series))
	for _, s := range out.Series {
		logf("  %-16s %3d posts  %6.1f likes  %5.1f comments", s.Series, s.Posts, s.AvgLikes, s.AvgComments)
	}
	return nil
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }

// cmdDoctor checks everything a run depends on and says plainly what is wrong.
func cmdDoctor(_ []string) error {
	failed := false
	check := func(ok bool, good, bad string) {
		if ok {
			logf("  ok    %s", good)
		} else {
			logf("  FAIL  %s", bad)
			failed = true
		}
	}

	q, err := load()
	check(err == nil, "data files load", fmt.Sprint("data files: ", err))
	if err == nil {
		problems := content.Validate(q.posts)
		check(len(problems) == 0, fmt.Sprintf("posts.json is valid (%d posts, %d unposted)", len(q.posts), content.Remaining(q.posts, q.posted)),
			fmt.Sprintf("posts.json has %d problem(s); run `quotebot validate`", len(problems)))
	}

	if os.Getenv("TMDB_KEY") == "" {
		logf("  note  TMDB_KEY is not set: film posts will use posters instead of stills")
	} else if err == nil {
		f := images.New(os.Getenv("TMDB_KEY"))
		imgs, _ := f.Fetch(content.Post{ID: "doctor", Kind: "film", Title: "Casablanca", Year: 1942})
		check(len(imgs) > 0, "TMDB key works", "TMDB returned no image for a test film: check the TMDB_KEY secret")
	}

	ig, err := client()
	if err != nil {
		check(false, "", err.Error())
	} else {
		acct, err := ig.Account()
		check(err == nil, fmt.Sprintf("Instagram token works: @%s, %d followers, %d posts", acct.Username, acct.Followers, acct.Posts),
			fmt.Sprint("Instagram: ", err))
		if err == nil {
			if used, err := ig.Quota(); err == nil {
				logf("  note  %d API posts used in the last 24 hours", used)
			}
		}
	}
	if failed {
		return errors.New("one or more checks failed")
	}
	logf("Everything looks right.")
	return nil
}
