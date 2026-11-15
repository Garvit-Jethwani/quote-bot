package images

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

func jpegBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	_ = color.RGBA{}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// fake serves every source from one server. Each test registers the paths it needs.
type fake struct {
	t      *testing.T
	routes map[string]func(w http.ResponseWriter, r *http.Request)
	hits   []string
}

func newFake(t *testing.T) (*fake, *Fetcher) {
	f := &fake{t: t, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits = append(f.hits, r.URL.Path)
		if h, ok := f.routes[r.URL.Path]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	fetch := New("key")
	fetch.E = Endpoints{
		TMDBAPI: srv.URL + "/tmdb", TMDBImg: srv.URL + "/timg",
		AICAPI: srv.URL + "/aic", AICImg: srv.URL + "/iiif",
		ITunes: srv.URL + "/itunes", OpenLibAPI: srv.URL + "/ol", OpenLibImg: srv.URL + "/olimg",
		WikiAPI: srv.URL + "/wiki",
	}
	f.routes["/base"] = func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(srv.URL)) }
	return f, fetch
}

func (f *fake) json(path string, v any) {
	f.routes[path] = func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(v) }
}

func (f *fake) image(path string, w, h int) {
	f.routes[path] = func(rw http.ResponseWriter, r *http.Request) { rw.Write(jpegBytes(w, h)) }
}

type m = map[string]any

func TestFilmUsesStillsSortedByRatingAndSkipsLogos(t *testing.T) {
	f, fetch := newFake(t)
	f.json("/tmdb/search/movie", m{"results": []m{{"id": 1, "title": "Wrong Film"}, {"id": 7, "title": "Paterson"}}})
	f.json("/tmdb/movie/7/images", m{"backdrops": []m{
		{"file_path": "/logo.jpg", "iso_639_1": "en", "width": 1920, "vote_average": 9},
		{"file_path": "/small.jpg", "iso_639_1": nil, "width": 780, "vote_average": 9},
		{"file_path": "/b.jpg", "iso_639_1": nil, "width": 1920, "vote_average": 5.1},
		{"file_path": "/a.jpg", "iso_639_1": nil, "width": 1920, "vote_average": 5.6},
	}})
	f.image("/timg/a.jpg", 1920, 1080)
	f.image("/timg/b.jpg", 1920, 1080)
	imgs, credit := fetch.Fetch(content.Post{ID: "p", Kind: "film", Title: "Paterson", Year: 2016})
	if len(imgs) != 2 || credit != "Stills via TMDB." {
		t.Fatalf("got %d images, credit %q", len(imgs), credit)
	}
	for _, h := range f.hits {
		if strings.Contains(h, "logo") || strings.Contains(h, "small") {
			t.Errorf("fetched an unusable image: %s", h)
		}
	}
	if f.hits[len(f.hits)-2] != "/timg/a.jpg" {
		t.Errorf("the best-rated still should be first, hits: %v", f.hits)
	}
}

func TestFilmFallsBackToPoster(t *testing.T) {
	f, fetch := newFake(t)
	f.json("/tmdb/search/movie", m{"results": []m{{"id": 7, "title": "Aftersun", "poster_path": "/p.jpg"}}})
	f.json("/tmdb/movie/7/images", m{"backdrops": []m{}})
	f.image("/timg/p.jpg", 1000, 1500)
	imgs, credit := fetch.Fetch(content.Post{ID: "p", Kind: "film", Title: "Aftersun", Year: 2022})
	if len(imgs) != 1 || credit != "Poster via TMDB." {
		t.Fatalf("got %d images, credit %q", len(imgs), credit)
	}
}

func TestSeriesUsesTheEpisodeThenTheShow(t *testing.T) {
	f, fetch := newFake(t)
	f.json("/tmdb/search/tv", m{"results": []m{{"id": 9, "name": "The Bear"}}})
	f.json("/tmdb/tv/9/season/1/episode/7/images", m{"stills": []m{{"file_path": "/e.jpg", "iso_639_1": nil, "width": 1920}}})
	f.json("/tmdb/tv/9/images", m{"backdrops": []m{{"file_path": "/show.jpg", "iso_639_1": nil, "width": 1920}}})
	f.image("/timg/e.jpg", 1920, 1080)
	f.image("/timg/show.jpg", 1920, 1080)
	post := content.Post{ID: "a", Kind: "series", Title: "The Bear", Year: 2022, Episode: []int{1, 7}}
	if imgs, _ := fetch.Fetch(post); len(imgs) != 1 || !strings.Contains(strings.Join(f.hits, " "), "/timg/e.jpg") {
		t.Fatalf("expected the episode still, hits: %v", f.hits)
	}
	f.hits = nil
	post.ID, post.Episode = "b", []int{2, 2} // no stills for this episode
	if imgs, _ := fetch.Fetch(post); len(imgs) != 1 || !strings.Contains(strings.Join(f.hits, " "), "/timg/show.jpg") {
		t.Fatalf("expected the show image, hits: %v", f.hits)
	}
}

func TestNoKeyMeansApplePosterWithTheRightYear(t *testing.T) {
	f, fetch := newFake(t)
	fetch.TMDBKey = ""
	base := fetch.E.ITunes[:len(fetch.E.ITunes)-len("/itunes")]
	f.json("/itunes", m{"results": []m{
		{"trackName": "Paterson", "releaseDate": "1999-01-01", "artworkUrl100": base + "/art/old/100x100bb.jpg"},
		{"trackName": "Paterson", "releaseDate": "2016-12-28", "artworkUrl100": base + "/art/new/100x100bb.jpg"},
	}})
	f.image("/art/new/1200x1200bb.jpg", 800, 1200)
	f.image("/art/old/1200x1200bb.jpg", 800, 1200)
	imgs, credit := fetch.Fetch(content.Post{ID: "p", Kind: "film", Title: "Paterson", Year: 2016})
	if len(imgs) != 1 || credit != "Poster via Apple TV." {
		t.Fatalf("got %d images, credit %q", len(imgs), credit)
	}
	if strings.Contains(strings.Join(f.hits, " "), "/art/old/") {
		t.Error("picked the film from the wrong year")
	}
}

func TestAlbumMustMatchTheArtist(t *testing.T) {
	f, fetch := newFake(t)
	base := fetch.E.ITunes[:len(fetch.E.ITunes)-len("/itunes")]
	f.json("/itunes", m{"results": []m{
		{"collectionName": "Pink Moon (Tribute)", "artistName": "Somebody Else", "artworkUrl100": base + "/art/wrong/100x100bb.jpg"},
		{"collectionName": "Pink Moon", "artistName": "Nick Drake", "artworkUrl100": base + "/art/right/100x100bb.jpg"},
	}})
	f.image("/art/right/1200x1200bb.jpg", 1200, 1200)
	f.image("/art/wrong/1200x1200bb.jpg", 1200, 1200)
	imgs, _ := fetch.Fetch(content.Post{ID: "p", Kind: "music", Title: "Pink Moon", Artist: "Nick Drake", Year: 1972})
	if len(imgs) != 1 || strings.Contains(strings.Join(f.hits, " "), "/art/wrong/") {
		t.Fatalf("wrong album chosen, hits: %v", f.hits)
	}
}

func TestBookCoverAndArtworkAndWikipediaGuards(t *testing.T) {
	f, fetch := newFake(t)
	f.json("/ol/search.json", m{"docs": []m{{"title": "Other Book", "cover_i": 1}, {"title": "Stoner", "cover_i": 42}}})
	f.image("/olimg/b/id/42-L.jpg", 330, 500)
	if imgs, credit := fetch.Fetch(content.Post{ID: "b", Kind: "book", Title: "Stoner", Author: "John Williams"}); len(imgs) != 1 || credit != "Cover via Open Library." {
		t.Fatalf("book cover: %d images, %q", len(imgs), credit)
	}

	// A painting must be public domain, match the title and match the artist.
	f.json("/aic/artworks/search", m{"data": []m{
		{"title": "Study for The Bedroom", "image_id": "wrong", "is_public_domain": true, "artist_title": "Somebody Else"},
		{"title": "The Bedroom", "image_id": "locked", "is_public_domain": false, "artist_title": "Vincent van Gogh"},
		{"title": "The Bedroom", "image_id": "right", "is_public_domain": true, "artist_title": "Vincent van Gogh"},
	}})
	f.image("/iiif/right/full/1686,/0/default.jpg", 1686, 1290)
	art := content.Post{ID: "a", Kind: "art", Title: "The Bedroom", Artist: "Vincent van Gogh", Year: 1889, Image: &content.ImageRef{AIC: "The Bedroom"}}
	if imgs, credit := fetch.Fetch(art); len(imgs) != 1 || !strings.Contains(credit, "public domain") {
		t.Fatalf("artwork: %d images, %q", len(imgs), credit)
	}

	// Wikipedia: a page about a town must not be used for a film.
	fetch.TMDBKey = ""
	f.json("/itunes", m{"results": []m{}})
	f.json("/wiki/page/summary/Columbus_(2017_film)", m{"type": "standard", "description": "City in Indiana, United States", "originalimage": m{"source": "x"}})
	if imgs, _ := fetch.Fetch(content.Post{ID: "c", Kind: "film", Title: "Columbus", Year: 2017}); len(imgs) != 0 {
		t.Fatal("used a picture from the wrong Wikipedia page")
	}
	base := fetch.E.WikiAPI[:len(fetch.E.WikiAPI)-len("/wiki")]
	f.json("/wiki/page/summary/Still_Walking_(film)", m{"type": "standard", "description": "2008 Japanese film", "originalimage": m{"source": base + "/wimg/sw.jpg"}})
	f.image("/wimg/sw.jpg", 260, 385)
	if imgs, credit := fetch.Fetch(content.Post{ID: "s", Kind: "film", Title: "Still Walking", Year: 2008}); len(imgs) != 1 || credit != "Image via Wikipedia." {
		t.Fatalf("wikipedia: %d images, %q", len(imgs), credit)
	}
}

func TestEverythingDownStillReturns(t *testing.T) {
	_, fetch := newFake(t)
	for _, kind := range []string{"film", "series", "art", "music", "book", "comedy"} {
		post := content.Post{ID: kind, Kind: kind, Title: "X", Artist: "A B", Year: 2000, Image: &content.ImageRef{AIC: "X"}}
		if imgs, _ := fetch.Fetch(post); len(imgs) != 0 {
			t.Errorf("%s: expected no images", kind)
		}
	}
}
