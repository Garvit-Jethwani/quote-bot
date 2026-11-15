package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

const testPosts = `[
 {"id":"deck","kind":"film","label":"SCENE STUDY","title":"A Film","year":2000,"director":"A Director","hook":"A hook.",
  "slides":[{"k":"ONE","h":"Head","b":"Body text."},{"k":"TWO","h":"Head","b":"Body text."},{"q":"A line","by":"Someone"}],
  "caption":"a caption","tags":"#afilm"},
 {"id":"card","kind":"comedy","label":"ONE-LINER","title":"A Comic","quote":"A joke.","slides":[],"caption":"ha","tags":"#acomic"}
]`

// workspace makes a temporary repository folder and a fake Instagram that
// also serves the drawn files, the way GitHub does in production.
func workspace(t *testing.T) (dir string, igCalls *[]string, fail *bool) {
	t.Helper()
	dir = t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("posts.json", testPosts)
	write("config.json", `{"handle":"@test","swipe_hint":"SWIPE","max_hashtags":5,"hashtags":{"film":"#cinephile"}}`)
	write("posted.json", "[]")

	var calls []string
	failing := false
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/files/") {
			http.ServeFile(w, r, filepath.Join(dir, strings.TrimPrefix(r.URL.Path, "/files/")))
			return
		}
		r.ParseForm()
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Form.Get("media_type"))
		switch {
		case failing:
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "refused", "code": 100}})
		case r.Method == "POST" && r.URL.Path == "/1789/media":
			n++
			json.NewEncoder(w).Encode(map[string]any{"id": "C" + string(rune('0'+n))})
		case r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]any{"id": "M1"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"status_code": "FINISHED"})
		}
	}))
	t.Cleanup(srv.Close)

	old, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(old) })
	t.Setenv("NO_IMAGES", "1")
	t.Setenv("IG_TOKEN", "tok")
	t.Setenv("IG_USER_ID", "1789")
	t.Setenv("IG_API_BASE", srv.URL)
	t.Setenv("IMAGE_BASE_URL", srv.URL+"/files")
	return dir, &calls, &failing
}

func postedIDs(t *testing.T) []string {
	t.Helper()
	entries, err := content.LoadPosted("posted.json")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID+":"+e.Type)
	}
	return ids
}

func TestCarouselRenderPublishRecord(t *testing.T) {
	_, calls, fail := workspace(t)
	if err := cmdRender([]string{"-id", "deck"}); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("out/*-deck-*.jpg")
	if len(files) != 4 {
		t.Fatalf("drew %d slides, want 4", len(files))
	}
	if got := postedIDs(t); len(got) != 0 {
		t.Fatalf("render must not record the post, got %v", got)
	}

	// A failed publish leaves the post unrecorded, so the next run retries it.
	*fail = true
	if err := cmdPublish(nil); err == nil {
		t.Fatal("publish should have failed")
	}
	if got := postedIDs(t); len(got) != 0 {
		t.Fatalf("a failed publish was recorded: %v", got)
	}

	*fail = false
	*calls = nil
	if err := cmdPublish(nil); err != nil {
		t.Fatal(err)
	}
	if got := postedIDs(t); len(got) != 1 || got[0] != "deck:post" {
		t.Fatalf("posted.json: %v", got)
	}
	if _, err := os.Stat("pending.json"); !os.IsNotExist(err) {
		t.Error("pending.json should be removed after publishing")
	}
	if joined := strings.Join(*calls, "\n"); strings.Count(joined, "POST /1789/media ") != 5 || !strings.Contains(joined, "CAROUSEL") {
		t.Errorf("unexpected API calls:\n%s", joined)
	}
	if err := cmdPublish(nil); err == nil {
		t.Error("publishing twice should fail: nothing is pending")
	}
}

func TestOneLinerIsASingleImage(t *testing.T) {
	_, calls, _ := workspace(t)
	if err := cmdRender([]string{"-id", "card"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdPublish(nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(*calls, "\n"), "CAROUSEL") {
		t.Error("a one-liner should be posted as a single image")
	}
}

func TestReel(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	_, calls, _ := workspace(t)
	if err := cmdRender([]string{"-reel"}); err != nil { // must choose the multi-slide post
		t.Fatal(err)
	}
	videos, _ := filepath.Glob("out/*-deck-reel.mp4")
	covers, _ := filepath.Glob("out/*-deck-cover.jpg")
	if len(videos) != 1 || len(covers) != 1 {
		t.Fatalf("videos %v, covers %v", videos, covers)
	}
	if err := cmdPublish(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(*calls, "\n"), "REELS") {
		t.Errorf("expected a REELS container, calls: %v", *calls)
	}
	if got := postedIDs(t); len(got) != 1 || got[0] != "deck:reel" {
		t.Fatalf("posted.json: %v", got)
	}
	if err := cmdRender([]string{"-reel"}); err == nil {
		t.Error("no multi-slide post is left, so a second Reel should be refused")
	}
}

func TestFallsBackToQuotesWhenTheQueueIsEmpty(t *testing.T) {
	dir, _, _ := workspace(t)
	os.WriteFile(filepath.Join(dir, "quotes.json"), []byte(`[{"type":"book","text":"A line.","source":"A Book","by":"An Author"}]`), 0o644)
	os.WriteFile(filepath.Join(dir, "posted.json"), []byte(`[{"id":"deck","type":"post","date":"x"},{"id":"card","type":"post","date":"x"}]`), 0o644)
	if err := cmdRender(nil); err != nil {
		t.Fatal(err)
	}
	if err := cmdPublish(nil); err != nil {
		t.Fatal(err)
	}
	if got := postedIDs(t); len(got) != 3 || !strings.HasSuffix(got[2], ":book") {
		t.Fatalf("posted.json: %v", got)
	}
}

func TestValidateCommand(t *testing.T) {
	dir, _, _ := workspace(t)
	if err := cmdValidate(nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "posts.json"), []byte(strings.Replace(testPosts, "a caption", "a — caption", 1)), 0o644)
	if err := cmdValidate(nil); err == nil {
		t.Error("a dash in a caption should fail validation")
	}
}
