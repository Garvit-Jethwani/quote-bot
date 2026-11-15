package instagram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type call struct {
	Method, Path string
	Form         url.Values
}

// fakeAPI imitates the publishing endpoints and records every call.
type fakeAPI struct {
	calls     []call
	polls     map[string]int
	rejectAlt bool // refuse containers that carry alt_text
	failFirst int  // answer this many calls with a transient error
	n         int
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	form := r.Form
	if form.Get("access_token") != "tok" {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad token", "code": 190}})
		return
	}
	form.Del("access_token")
	f.calls = append(f.calls, call{r.Method, r.URL.Path, form})
	if f.failFirst > 0 {
		f.failFirst--
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "try again", "code": 2, "is_transient": true}})
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/me":
		json.NewEncoder(w).Encode(map[string]any{"user_id": "1789"})
	case r.Method == "POST" && r.URL.Path == "/1789/media":
		if f.rejectAlt && form.Get("alt_text") != "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Unsupported parameter alt_text", "code": 100}})
			return
		}
		f.n++
		json.NewEncoder(w).Encode(map[string]any{"id": "C" + string(rune('0'+f.n))})
	case r.Method == "POST" && r.URL.Path == "/1789/media_publish":
		json.NewEncoder(w).Encode(map[string]any{"id": "M" + form.Get("creation_id")})
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/C"):
		f.polls[r.URL.Path]++
		status := "IN_PROGRESS"
		if f.polls[r.URL.Path] > 1 {
			status = "FINISHED"
		}
		json.NewEncoder(w).Encode(map[string]any{"status_code": status})
	default:
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "unknown path", "code": 100}})
	}
}

func setup(t *testing.T) (*fakeAPI, *Client) {
	api := &fakeAPI{polls: map[string]int{}}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	c := New("tok", "1789")
	c.Base = srv.URL
	c.Poll = time.Millisecond
	return api, c
}

func posts(calls []call) []call {
	var out []call
	for _, c := range calls {
		if c.Method == "POST" {
			out = append(out, c)
		}
	}
	return out
}

func TestBaseFollowsTheTokenType(t *testing.T) {
	if got := New("EAAZabc", "").Base; got != "https://graph.facebook.com" {
		t.Errorf("EAA token: %s", got)
	}
	if got := New("IGAAabc", "").Base; got != "https://graph.instagram.com" {
		t.Errorf("IG token: %s", got)
	}
}

func TestPublishCarousel(t *testing.T) {
	api, c := setup(t)
	id, err := c.PublishCarousel([]string{"u1", "u2", "u3"}, "cap", []string{"a1", "a2", "a3"})
	if err != nil {
		t.Fatal(err)
	}
	p := posts(api.calls)
	if len(p) != 5 || id != "MC4" {
		t.Fatalf("id %s, %d POSTs", id, len(p))
	}
	for i := 0; i < 3; i++ {
		if p[i].Form.Get("is_carousel_item") != "true" || p[i].Form.Get("alt_text") == "" || p[i].Form.Get("caption") != "" {
			t.Errorf("child %d: %v", i, p[i].Form)
		}
	}
	if p[3].Form.Get("media_type") != "CAROUSEL" || p[3].Form.Get("children") != "C1,C2,C3" || p[3].Form.Get("caption") != "cap" {
		t.Errorf("parent: %v", p[3].Form)
	}
	if p[4].Path != "/1789/media_publish" || p[4].Form.Get("creation_id") != "C4" {
		t.Errorf("publish: %v", p[4])
	}
}

func TestPublishImageDropsAltTextIfRefused(t *testing.T) {
	api, c := setup(t)
	api.rejectAlt = true
	if _, err := c.PublishImage("u", "cap", "alt"); err != nil {
		t.Fatal(err)
	}
	p := posts(api.calls)
	if len(p) != 3 || p[0].Form.Get("alt_text") == "" || p[1].Form.Get("alt_text") != "" {
		t.Fatalf("expected one try with alt text and one without, got %v", p)
	}
}

func TestPublishReel(t *testing.T) {
	api, c := setup(t)
	if _, err := c.PublishReel("v.mp4", "cover.jpg", "cap"); err != nil {
		t.Fatal(err)
	}
	f := posts(api.calls)[0].Form
	if f.Get("media_type") != "REELS" || f.Get("video_url") != "v.mp4" || f.Get("cover_url") != "cover.jpg" || f.Get("share_to_feed") != "true" {
		t.Errorf("reel container: %v", f)
	}
}

func TestTransientErrorsAreRetriedAndRefusalsAreNot(t *testing.T) {
	api, c := setup(t)
	api.failFirst = 2
	if _, err := c.PublishImage("u", "cap", ""); err != nil {
		t.Fatalf("should have recovered: %v", err)
	}

	api2, c2 := setup(t)
	c2.Token = "wrong"
	_, err := c2.PublishImage("u", "cap", "")
	if err == nil || len(api2.calls) != 0 {
		t.Fatal("a bad token should fail")
	}
	if strings.Contains(err.Error(), "wrong") || strings.Contains(err.Error(), "access_token") {
		t.Errorf("the error leaks the token: %v", err)
	}
}

func TestUserLookup(t *testing.T) {
	_, c := setup(t)
	c.UserID = ""
	if id, err := c.User(); err != nil || id != "1789" {
		t.Fatalf("got %q, %v", id, err)
	}
	_, c = setup(t)
	c.UserID, c.Token = "", "EAAtok"
	if _, err := c.User(); err == nil || !strings.Contains(err.Error(), "IG_USER_ID") {
		t.Errorf("expected a clear message about IG_USER_ID, got %v", err)
	}
}
