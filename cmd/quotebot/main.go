// Command quotebot draws posts from posts.json and publishes them to
// Instagram.
//
//	quotebot render [-id ID] [-reel]   pick a post and draw it into out/
//	quotebot publish                   publish what render drew, then record it
//	quotebot preview [-id ID]          draw posts into preview/ for a look
//	quotebot validate                  check posts.json; non-zero exit on problems
//	quotebot status                    show what is left in the queue
//	quotebot stats                     save likes and comments per post to stats.json
//	quotebot doctor                    check the secrets and the connection to Instagram
//
// Secrets come from the environment: IG_TOKEN (required to publish),
// IG_USER_ID and TMDB_KEY (optional). NO_IMAGES=1 draws text-only slides.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/Garvit-Jethwani/quote-bot/internal/images"
	"github.com/Garvit-Jethwani/quote-bot/internal/instagram"
	"github.com/Garvit-Jethwani/quote-bot/internal/reel"
	"github.com/Garvit-Jethwani/quote-bot/internal/render"
)

const (
	keepDays    = 14 // how long drawn files stay in out/
	lowQueue    = 10 // warn when this few posts are left
	stampLayout = "20060102-1504"
)

// pending is what render hands to publish, saved as pending.json.
type pending struct {
	Images  []string      `json:"images"`
	Alts    []string      `json:"alts,omitempty"`
	Video   string        `json:"video,omitempty"`
	Cover   string        `json:"cover,omitempty"`
	Caption string        `json:"caption"`
	Entry   content.Entry `json:"entry"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "render":
		err = cmdRender(args)
	case "publish":
		err = cmdPublish(args)
	case "preview":
		err = cmdPreview(args)
	case "validate":
		err = cmdValidate(args)
	case "status":
		err = cmdStatus(args)
	case "stats":
		err = cmdStats(args)
	case "doctor":
		err = cmdDoctor(args)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: quotebot render [-id ID] [-reel] | publish | preview [-id ID] | validate | status | stats | doctor")
	os.Exit(2)
}

func logf(format string, args ...any) { fmt.Printf(format+"\n", args...) }

// queue is everything loaded from the repository's data files.
type queue struct {
	cfg    content.Config
	posts  []content.Post
	quotes []content.Quote
	posted []content.Entry
}

func load() (queue, error) {
	var q queue
	var err error
	if q.cfg, err = content.LoadConfig("config.json"); err != nil {
		return q, err
	}
	if q.posts, err = content.LoadPosts("posts.json"); err != nil {
		return q, err
	}
	if q.quotes, err = content.LoadQuotes("quotes.json"); err != nil {
		return q, err
	}
	q.posted, err = content.LoadPosted("posted.json")
	return q, err
}

func fetcher() *images.Fetcher {
	f := images.New(os.Getenv("TMDB_KEY"))
	f.Log = logf
	return f
}

func stills(f *images.Fetcher, p content.Post) ([]image.Image, string) {
	if os.Getenv("NO_IMAGES") != "" {
		return nil, ""
	}
	return f.Fetch(p)
}

// ---------------------------------------------------------------- render

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	id := fs.String("id", "", "post id from posts.json (default: a random unposted one)")
	asReel := fs.Bool("reel", false, "make a vertical video instead of a carousel")
	fs.Parse(args)

	q, err := load()
	if err != nil {
		return err
	}
	if problems := content.Validate(q.posts); len(problems) > 0 {
		logf("::warning::posts.json has %d problem(s); run `quotebot validate`", len(problems))
	}
	now := time.Now().UTC()
	rng := rand.New(rand.NewSource(now.UnixNano()))
	out := pending{}

	post, ok := content.Next(q.posts, q.posted, *id, *asReel, rng)
	switch {
	case *id != "" && !ok:
		return fmt.Errorf("no post with id %q in posts.json", *id)
	case ok:
		cfg := q.cfg
		if *asReel {
			cfg.SwipeHint = "" // nothing to swipe in a video
		}
		imgs, credit := stills(fetcher(), post)
		slides := render.Post(post, cfg, imgs)
		stem := filepath.Join("out", now.Format(stampLayout)+"-"+post.ID)
		out.Caption = content.Caption(post, q.cfg, credit)
		out.Entry = content.Entry{ID: post.ID, Type: "post", Date: now.Format("2006-01-02")}
		if *asReel {
			if len(slides) < 2 {
				return fmt.Errorf("%s has a single slide; a Reel needs at least two", post.ID)
			}
			th := render.ThemeFor(post.Kind)
			out.Video = stem + "-reel.mp4"
			secs, err := reel.Build(slides, reel.Durations(post, len(slides)), th.BG, th.Accent, out.Video)
			if err != nil {
				return err
			}
			out.Cover = stem + "-cover.jpg"
			if err := render.SaveJPEG(reel.Frame(slides[0], th.BG), out.Cover); err != nil {
				return err
			}
			out.Entry.Type = "reel"
			logf("Drew %s as a %.0f second Reel", post.Title, secs)
		} else {
			for i, s := range slides {
				path := fmt.Sprintf("%s-%02d.jpg", stem, i+1)
				if err := render.SaveJPEG(s, path); err != nil {
					return err
				}
				out.Images = append(out.Images, path)
				out.Alts = append(out.Alts, content.AltText(post, i))
			}
			logf("Drew %s (%d slide(s))", post.Title, len(slides))
		}
		if left := content.Remaining(q.posts, q.posted) - 1; left <= lowQueue {
			logf("::warning::Only %d unposted posts left in posts.json. Add more soon.", left)
		}
	case *asReel:
		return errors.New("no unposted multi-slide post left to turn into a Reel")
	default: // the queue is empty: fall back to a quote card so the account stays alive
		quote, ok := content.NextQuote(q.quotes, q.posted, rng)
		if !ok {
			return errors.New("nothing left to post: posts.json is used up and quotes.json is empty")
		}
		path := filepath.Join("out", now.Format(stampLayout)+"-"+content.QuoteID(quote)+".jpg")
		if err := render.SaveJPEG(render.QuoteCard(quote, q.cfg), path); err != nil {
			return err
		}
		out.Images = []string{path}
		out.Alts = []string{"Quote: " + quote.Text}
		out.Caption = content.QuoteCaption(quote, q.cfg)
		out.Entry = content.Entry{ID: content.QuoteID(quote), Type: quote.Type, Date: now.Format("2006-01-02")}
		logf("::warning::posts.json is used up; drew a fallback quote from %s", quote.Source)
	}

	prune(now)
	data, _ := json.MarshalIndent(out, "", "  ")
	return os.WriteFile("pending.json", data, 0o644)
}

// prune deletes drawn files older than keepDays so the repository stays small.
func prune(now time.Time) {
	cutoff := now.AddDate(0, 0, -keepDays).Format("20060102")
	files, _ := filepath.Glob("out/*")
	for _, f := range files {
		name := filepath.Base(f)
		if len(name) > 8 && name[:8] < cutoff && (strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".mp4")) {
			os.Remove(f)
		}
	}
}

// ---------------------------------------------------------------- publish

// publicURLs lists the addresses a pushed file can be downloaded from. The
// CDN copy is served with a proper media type, which video needs.
func publicURLs(path string, video bool) ([]string, error) {
	path = filepath.ToSlash(path)
	if base := os.Getenv("IMAGE_BASE_URL"); base != "" {
		return []string{strings.TrimRight(base, "/") + "/" + path}, nil
	}
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return nil, errors.New("set GITHUB_REPOSITORY (owner/repo) or IMAGE_BASE_URL")
	}
	sha, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	rev := strings.TrimSpace(string(sha))
	raw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, rev, path)
	cdn := fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s@%s/%s", repo, rev, path)
	if video {
		return []string{cdn, raw}, nil
	}
	return []string{raw, cdn}, nil
}

// reachable waits until one of a file's addresses answers, and returns it.
func reachable(path string, video bool) (string, error) {
	urls, err := publicURLs(path, video)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	wait := 4 * time.Second
	if os.Getenv("IMAGE_BASE_URL") != "" {
		wait = 200 * time.Millisecond
	}
	for attempt := 0; attempt < 30; attempt++ {
		for _, u := range urls {
			resp, err := client.Head(u)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return u, nil
				}
			}
		}
		time.Sleep(wait)
	}
	return "", fmt.Errorf("%s is not publicly reachable (tried %s); is the repository public and was the file pushed?", path, urls[0])
}

func cmdPublish(_ []string) error {
	token := os.Getenv("IG_TOKEN")
	if strings.TrimSpace(token) == "" {
		return errors.New("IG_TOKEN is not set; add it as a repository secret")
	}
	data, err := os.ReadFile("pending.json")
	if err != nil {
		return errors.New("nothing to publish: run `quotebot render` first")
	}
	var p pending
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("pending.json: %w", err)
	}

	ig := instagram.New(token, os.Getenv("IG_USER_ID"))
	ig.Log = logf
	if os.Getenv("IG_API_BASE") != "" {
		ig.Poll = 50 * time.Millisecond
	}

	var media string
	switch {
	case p.Video != "":
		video, err := reachable(p.Video, true)
		if err != nil {
			return err
		}
		cover := ""
		if p.Cover != "" {
			cover, _ = reachable(p.Cover, false) // a missing cover is not fatal
		}
		if media, err = ig.PublishReel(video, cover, p.Caption); err != nil {
			return err
		}
		logf("Published a Reel. Media id: %s", media)
	case len(p.Images) > 0:
		var urls []string
		for _, img := range p.Images {
			u, err := reachable(img, false)
			if err != nil {
				return err
			}
			urls = append(urls, u)
		}
		if len(urls) == 1 {
			alt := ""
			if len(p.Alts) > 0 {
				alt = p.Alts[0]
			}
			media, err = ig.PublishImage(urls[0], p.Caption, alt)
		} else {
			media, err = ig.PublishCarousel(urls, p.Caption, p.Alts)
		}
		if err != nil {
			return err
		}
		logf("Published %d slide(s). Media id: %s", len(urls), media)
	default:
		return errors.New("pending.json has nothing in it")
	}

	// Record the post only now that Instagram has accepted it, so a failed
	// publish is retried on the next run instead of being lost.
	posted, err := content.LoadPosted("posted.json")
	if err != nil {
		return err
	}
	p.Entry.Media = media
	if err := content.SavePosted("posted.json", append(posted, p.Entry)); err != nil {
		return err
	}
	return os.Remove("pending.json")
}

// ---------------------------------------------------------------- preview, validate, status

func cmdPreview(args []string) error {
	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	id := fs.String("id", "", "only this post")
	dir := fs.String("out", "preview", "folder to draw into")
	withImages := fs.Bool("images", false, "fetch real images (slower; needs network)")
	fs.Parse(args)

	q, err := load()
	if err != nil {
		return err
	}
	f := fetcher()
	n := 0
	for _, p := range q.posts {
		if *id != "" && p.ID != *id {
			continue
		}
		var imgs []image.Image
		if *withImages {
			imgs, _ = stills(f, p)
		}
		for i, s := range render.Post(p, q.cfg, imgs) {
			if err := render.SaveJPEG(s, filepath.Join(*dir, p.ID, fmt.Sprintf("%s-%02d.jpg", p.ID, i+1))); err != nil {
				return err
			}
		}
		n++
	}
	if *id != "" && n == 0 {
		return fmt.Errorf("no post with id %q", *id)
	}
	logf("Drew %d post(s) into %s/", n, *dir)
	return nil
}

func cmdValidate(_ []string) error {
	posts, err := content.LoadPosts("posts.json")
	if err != nil {
		return err
	}
	if _, err := content.LoadConfig("config.json"); err != nil {
		return err
	}
	problems := content.Validate(posts)
	for _, p := range problems {
		fmt.Println(" -", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s) in posts.json", len(problems))
	}
	logf("posts.json is valid: %d posts", len(posts))
	return nil
}

func cmdStatus(_ []string) error {
	q, err := load()
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, e := range q.posted {
		used[e.ID] = true
	}
	left := map[string]int{}
	total := map[string]int{}
	for _, p := range q.posts {
		total[p.Label]++
		if !used[p.ID] {
			left[p.Label]++
		}
	}
	labels := make([]string, 0, len(total))
	for l := range total {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	logf("%-16s %6s %6s", "SERIES", "LEFT", "TOTAL")
	for _, l := range labels {
		logf("%-16s %6d %6d", l, left[l], total[l])
	}
	logf("%-16s %6d %6d", "ALL", content.Remaining(q.posts, q.posted), len(q.posts))
	logf("Published so far: %d", len(q.posted))
	return nil
}
