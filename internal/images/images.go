// Package images finds a picture for a post. Each kind of post has a chain
// of sources tried in order; the first that answers wins, and a post with no
// picture is simply drawn as text.
//
//	film, series  TMDB stills -> TMDB poster -> Apple poster (films) -> Wikipedia
//	art           Art Institute of Chicago (public domain) -> Wikipedia
//	music         Apple Music album cover -> Wikipedia
//	book          Open Library cover -> Wikipedia
package images

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif" // decoders for whatever a source serves
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	_ "golang.org/x/image/webp"
)

// Endpoints are the base URLs of every source. Tests point them at a local server.
type Endpoints struct {
	TMDBAPI, TMDBImg       string
	AICAPI, AICImg         string
	ITunes                 string
	OpenLibAPI, OpenLibImg string
	WikiAPI                string
}

// DefaultEndpoints returns the real services, each overridable by an
// environment variable of the same name as the Python version used.
func DefaultEndpoints() Endpoints {
	env := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return strings.TrimRight(v, "/")
		}
		return def
	}
	return Endpoints{
		TMDBAPI:    env("TMDB_API_BASE", "https://api.themoviedb.org/3"),
		TMDBImg:    env("TMDB_IMG_BASE", "https://image.tmdb.org/t/p/w1280"),
		AICAPI:     env("AIC_API_BASE", "https://api.artic.edu/api/v1"),
		AICImg:     env("AIC_IMG_BASE", "https://www.artic.edu/iiif/2"),
		ITunes:     env("ITUNES_API_BASE", "https://itunes.apple.com/search"),
		OpenLibAPI: env("OPENLIB_API_BASE", "https://openlibrary.org"),
		OpenLibImg: env("OPENLIB_IMG_BASE", "https://covers.openlibrary.org"),
		WikiAPI:    env("WIKI_API_BASE", "https://en.wikipedia.org/api/rest_v1"),
	}
}

// Fetcher looks up images.
type Fetcher struct {
	HTTP    *http.Client
	TMDBKey string
	E       Endpoints
	Log     func(format string, args ...any)

	tmdb map[string]*tmdbTitle
}

// New returns a Fetcher for the real services.
func New(tmdbKey string) *Fetcher {
	return &Fetcher{
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		TMDBKey: strings.TrimSpace(tmdbKey),
		E:       DefaultEndpoints(),
		Log:     func(string, ...any) {},
	}
}

type source struct {
	name string
	fn   func(*Fetcher, content.Post) ([]image.Image, string, error)
}

var chains = map[string][]source{
	"film":   {{"tmdb stills", (*Fetcher).tmdbStills}, {"tmdb poster", (*Fetcher).tmdbPoster}, {"apple poster", (*Fetcher).appleMovie}, {"wikipedia", (*Fetcher).wikipedia}},
	"series": {{"tmdb stills", (*Fetcher).tmdbStills}, {"tmdb poster", (*Fetcher).tmdbPoster}, {"wikipedia", (*Fetcher).wikipedia}},
	"art":    {{"art institute of chicago", (*Fetcher).aic}, {"wikipedia", (*Fetcher).wikipedia}},
	"music":  {{"apple album", (*Fetcher).appleAlbum}, {"wikipedia", (*Fetcher).wikipedia}},
	"book":   {{"open library", (*Fetcher).openLibrary}, {"wikipedia", (*Fetcher).wikipedia}},
}

// Fetch returns the images for a post and a credit line for them. It never
// fails: a source that errors is logged and skipped.
func (f *Fetcher) Fetch(p content.Post) ([]image.Image, string) {
	for _, s := range chains[p.Kind] {
		imgs, credit, err := s.fn(f, p)
		if err != nil {
			f.Log("images: %s failed for %s: %v", s.name, p.ID, err)
			continue
		}
		if len(imgs) > 0 {
			f.Log("images: %s <- %s (%d)", p.ID, s.name, len(imgs))
			return imgs, credit
		}
	}
	if len(chains[p.Kind]) > 0 {
		f.Log("images: nothing found for %s; drawing text only", p.ID)
	}
	return nil, ""
}

func (f *Fetcher) getJSON(base string, params url.Values, headers map[string]string, out any) error {
	u := base
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "carousel-bot/2.0")
	req.Header.Set("AIC-User-Agent", "carousel-bot")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return scrub(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// scrub removes the query string (which can hold an API key) from a URL error.
func scrub(err error) error {
	if ue, ok := err.(*url.Error); ok {
		if i := strings.Index(ue.URL, "?"); i >= 0 {
			ue.URL = ue.URL[:i]
		}
	}
	return err
}

// getImage downloads and decodes a picture. Tiny images are rejected.
func (f *Fetcher) getImage(u string) image.Image {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "carousel-bot/2.0")
	req.Header.Set("AIC-User-Agent", "carousel-bot")
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil
	}
	if b := img.Bounds(); b.Dx() < 200 || b.Dy() < 200 {
		return nil
	}
	return img
}

// same is a loose title match: one contains the other, ignoring case and punctuation.
func same(a, b string) bool {
	norm := func(s string) string {
		var sb strings.Builder
		for _, r := range strings.ToLower(s) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
				sb.WriteRune(r)
			}
		}
		return strings.TrimSpace(sb.String())
	}
	a, b = norm(a), norm(b)
	return a != "" && b != "" && (strings.Contains(a, b) || strings.Contains(b, a))
}

// ---- TMDB

type tmdbTitle struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Name          string `json:"name"`
	OriginalName  string `json:"original_name"`
	PosterPath    string `json:"poster_path"`
}

type tmdbShot struct {
	FilePath    string  `json:"file_path"`
	Lang        *string `json:"iso_639_1"`
	Width       int     `json:"width"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

func (f *Fetcher) tmdbGet(path string, params url.Values, out any) error {
	headers := map[string]string{}
	if params == nil {
		params = url.Values{}
	}
	if len(f.TMDBKey) > 40 { // a v4 read-access token
		headers["Authorization"] = "Bearer " + f.TMDBKey
	} else {
		params.Set("api_key", f.TMDBKey)
	}
	return f.getJSON(f.E.TMDBAPI+"/"+path, params, headers, out)
}

// tmdbFind looks up the film, or the show for a series post.
func (f *Fetcher) tmdbFind(p content.Post) (*tmdbTitle, error) {
	if t, ok := f.tmdb[p.ID]; ok {
		return t, nil
	}
	path, yearKey, year := "search/movie", "year", p.Year
	if p.Kind == "series" {
		path, yearKey = "search/tv", "first_air_date_year"
		if p.FirstAired != 0 {
			year = p.FirstAired
		}
	}
	var res struct {
		Results []tmdbTitle `json:"results"`
	}
	if err := f.tmdbGet(path, url.Values{"query": {p.Title}, yearKey: {fmt.Sprint(year)}}, &res); err != nil {
		return nil, err
	}
	if len(res.Results) == 0 {
		if err := f.tmdbGet(path, url.Values{"query": {p.Title}}, &res); err != nil {
			return nil, err
		}
	}
	var found *tmdbTitle
	for i, r := range res.Results {
		for _, name := range []string{r.Title, r.OriginalTitle, r.Name, r.OriginalName} {
			if strings.EqualFold(name, p.Title) {
				found = &res.Results[i]
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil && len(res.Results) > 0 {
		found = &res.Results[0]
	}
	if f.tmdb == nil {
		f.tmdb = map[string]*tmdbTitle{}
	}
	f.tmdb[p.ID] = found
	return found, nil
}

func (f *Fetcher) tmdbStills(p content.Post) ([]image.Image, string, error) {
	if f.TMDBKey == "" {
		return nil, "", nil
	}
	t, err := f.tmdbFind(p)
	if err != nil || t == nil {
		return nil, "", err
	}
	var shots []tmdbShot
	textless := url.Values{"include_image_language": {"null"}}
	if p.Kind == "series" {
		if len(p.Episode) == 2 { // frames from the exact episode, when TMDB has them
			var ep struct {
				Stills []tmdbShot `json:"stills"`
			}
			path := fmt.Sprintf("tv/%d/season/%d/episode/%d/images", t.ID, p.Episode[0], p.Episode[1])
			if f.tmdbGet(path, nil, &ep) == nil {
				shots = ep.Stills
			}
		}
		if len(shots) == 0 {
			var show struct {
				Backdrops []tmdbShot `json:"backdrops"`
			}
			if err := f.tmdbGet(fmt.Sprintf("tv/%d/images", t.ID), textless, &show); err != nil {
				return nil, "", err
			}
			shots = show.Backdrops
		}
	} else {
		var film struct {
			Backdrops []tmdbShot `json:"backdrops"`
		}
		if err := f.tmdbGet(fmt.Sprintf("movie/%d/images", t.ID), textless, &film); err != nil {
			return nil, "", err
		}
		shots = film.Backdrops
	}
	var usable []tmdbShot
	for _, s := range shots {
		if (s.Lang == nil || *s.Lang == "xx") && s.Width >= 1280 {
			usable = append(usable, s)
		}
	}
	sort.SliceStable(usable, func(i, j int) bool {
		if usable[i].VoteAverage != usable[j].VoteAverage {
			return usable[i].VoteAverage > usable[j].VoteAverage
		}
		return usable[i].VoteCount > usable[j].VoteCount
	})
	if p.SkipStills < len(usable) {
		usable = usable[p.SkipStills:]
	} else {
		usable = nil
	}
	var out []image.Image
	for _, s := range usable {
		if img := f.getImage(f.E.TMDBImg + s.FilePath); img != nil {
			out = append(out, img)
		}
		if len(out) == 2 {
			break
		}
	}
	return out, "Stills via TMDB.", nil
}

func (f *Fetcher) tmdbPoster(p content.Post) ([]image.Image, string, error) {
	if f.TMDBKey == "" {
		return nil, "", nil
	}
	t, err := f.tmdbFind(p)
	if err != nil || t == nil || t.PosterPath == "" {
		return nil, "", err
	}
	if img := f.getImage(f.E.TMDBImg + t.PosterPath); img != nil {
		return []image.Image{img}, "Poster via TMDB.", nil
	}
	return nil, "", nil
}

// ---- Apple (iTunes Search)

func (f *Fetcher) apple(p content.Post, entity string) ([]image.Image, error) {
	var res struct {
		Results []struct {
			TrackName      string `json:"trackName"`
			CollectionName string `json:"collectionName"`
			ArtistName     string `json:"artistName"`
			ReleaseDate    string `json:"releaseDate"`
			Artwork        string `json:"artworkUrl100"`
		} `json:"results"`
	}
	term := strings.TrimSpace(p.SearchTitle() + " " + p.Maker())
	if err := f.getJSON(f.E.ITunes, url.Values{"term": {term}, "entity": {entity}, "limit": {"10"}}, nil, &res); err != nil {
		return nil, err
	}
	first := ""
	if parts := strings.Fields(p.Maker()); len(parts) > 0 {
		first = strings.ToLower(parts[0])
	}
	for _, r := range res.Results {
		name := r.CollectionName
		if entity == "movie" {
			name = r.TrackName
		}
		if r.Artwork == "" || !same(name, p.SearchTitle()) {
			continue
		}
		if entity == "album" && first != "" && !strings.Contains(strings.ToLower(r.ArtistName), first) {
			continue
		}
		if entity == "movie" && !strings.HasPrefix(r.ReleaseDate, fmt.Sprint(p.Year)) {
			continue
		}
		if img := f.getImage(strings.Replace(r.Artwork, "100x100bb", "1200x1200bb", 1)); img != nil {
			return []image.Image{img}, nil
		}
	}
	return nil, nil
}

func (f *Fetcher) appleMovie(p content.Post) ([]image.Image, string, error) {
	imgs, err := f.apple(p, "movie")
	return imgs, "Poster via Apple TV.", err
}

func (f *Fetcher) appleAlbum(p content.Post) ([]image.Image, string, error) {
	imgs, err := f.apple(p, "album")
	return imgs, "Cover via Apple Music.", err
}

// ---- Open Library

func (f *Fetcher) openLibrary(p content.Post) ([]image.Image, string, error) {
	var res struct {
		Docs []struct {
			Title string `json:"title"`
			Cover int    `json:"cover_i"`
		} `json:"docs"`
	}
	params := url.Values{"title": {p.SearchTitle()}, "author": {p.Maker()}, "limit": {"10"}, "fields": {"title,author_name,cover_i"}}
	if err := f.getJSON(f.E.OpenLibAPI+"/search.json", params, nil, &res); err != nil {
		return nil, "", err
	}
	for _, d := range res.Docs {
		if d.Cover == 0 || !same(d.Title, p.SearchTitle()) {
			continue
		}
		if img := f.getImage(fmt.Sprintf("%s/b/id/%d-L.jpg", f.E.OpenLibImg, d.Cover)); img != nil {
			return []image.Image{img}, "Cover via Open Library.", nil
		}
	}
	return nil, "", nil
}

// ---- Art Institute of Chicago

func (f *Fetcher) aic(p content.Post) ([]image.Image, string, error) {
	if p.Image == nil || p.Image.AIC == "" {
		return nil, "", nil
	}
	var res struct {
		Data []struct {
			Title        string `json:"title"`
			ImageID      string `json:"image_id"`
			PublicDomain bool   `json:"is_public_domain"`
			Artist       string `json:"artist_title"`
		} `json:"data"`
	}
	params := url.Values{"q": {p.Image.AIC}, "limit": {"15"}, "fields": {"id,title,image_id,is_public_domain,artist_title"}}
	if err := f.getJSON(f.E.AICAPI+"/artworks/search", params, nil, &res); err != nil {
		return nil, "", err
	}
	parts := strings.Fields(p.Artist)
	surname := strings.ToLower(parts[len(parts)-1])
	for _, d := range res.Data {
		if d.ImageID == "" || !d.PublicDomain ||
			!strings.Contains(strings.ToLower(d.Title), strings.ToLower(p.Image.AIC)) ||
			!strings.Contains(strings.ToLower(d.Artist), surname) {
			continue
		}
		for _, size := range []string{"1686,", "843,"} {
			if img := f.getImage(fmt.Sprintf("%s/%s/full/%s/0/default.jpg", f.E.AICImg, d.ImageID, size)); img != nil {
				return []image.Image{img}, "Image: Art Institute of Chicago, public domain.", nil
			}
		}
	}
	return nil, "", nil
}

// ---- Wikipedia

var wikiWords = map[string][]string{
	"film":   {"film"},
	"series": {"series", "sitcom", "drama", "show"},
	"music":  {"album"},
	"art":    {"painting", "print", "woodblock"},
	"book":   {"novel", "book", "poem", "letters", "collection", "work by", "novella"},
}

func (f *Fetcher) wikipedia(p content.Post) ([]image.Image, string, error) {
	t := p.Title
	var names []string
	switch {
	case p.Wiki != "":
		names = []string{p.Wiki}
	case p.Kind == "film":
		names = []string{fmt.Sprintf("%s (%d film)", t, p.Year), t + " (film)", t}
	case p.Kind == "series":
		names = []string{t + " (TV series)", t}
	case p.Kind == "music":
		names = []string{fmt.Sprintf("%s (%s album)", t, p.Maker()), t + " (album)", t}
	case p.Kind == "book":
		names = []string{t + " (novel)", t + " (book)", t, p.SearchTitle()}
	default:
		names = []string{t + " (painting)", t}
	}
	for _, name := range names {
		var page struct {
			Type        string `json:"type"`
			Description string `json:"description"`
			Original    *struct {
				Source string `json:"source"`
			} `json:"originalimage"`
			Thumb *struct {
				Source string `json:"source"`
			} `json:"thumbnail"`
		}
		u := f.E.WikiAPI + "/page/summary/" + url.PathEscape(strings.ReplaceAll(name, " ", "_"))
		if f.getJSON(u, nil, nil, &page) != nil || page.Type != "standard" {
			continue
		}
		about := strings.ToLower(page.Description)
		match := false
		for _, w := range wikiWords[p.Kind] {
			match = match || strings.Contains(about, w)
		}
		if !match { // the wrong page: a town, a person, a disambiguation
			continue
		}
		src := ""
		if page.Original != nil {
			src = page.Original.Source
		} else if page.Thumb != nil {
			src = page.Thumb.Source
		}
		if src == "" {
			continue
		}
		if img := f.getImage(src); img != nil {
			return []image.Image{img}, "Image via Wikipedia.", nil
		}
	}
	return nil, "", nil
}
