package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

func TestMotion(t *testing.T) {
	cfg := content.Config{Handle: "@x", Outro: map[string]string{"SCENE STUDY": "Send this on."}}
	p := content.Post{ID: "f", Kind: "film", Label: "SCENE STUDY", Title: "T", Year: 2000, Director: "D", Hook: "A hook line",
		Slides: []content.Slide{{Kicker: "K", Head: "Heading", Body: "First sentence here. Second sentence."}, {Quote: "A line", By: "someone"}, {Text: "A page of story."}}}
	for _, stills := range [][]image.Image{nil, {solid(1920, 1080, color.RGBA{200, 60, 40, 255})}, {solid(1000, 1500, color.RGBA{40, 60, 200, 255})}} {
		clip := Motion(p, cfg, stills)
		if got, want := len(clip.scenes), 5; got != want { // cover, three slides, closing
			t.Fatalf("%d scenes, want %d", got, want)
		}
		if clip.Seconds() < 10 || clip.Seconds() > 40 {
			t.Errorf("length %.1fs is outside a sensible range", clip.Seconds())
		}
		for _, i := range []int{0, clip.Frames() / 2, clip.Frames() - 1} {
			if b := clip.Frame(i).Bounds(); b.Dx() != RW || b.Dy() != RH {
				t.Fatalf("frame is %dx%d", b.Dx(), b.Dy())
			}
		}
	}
	if got := firstSentence("One short thing happened here. Then another."); got != "One short thing happened here." {
		t.Errorf("firstSentence: %q", got)
	}
}
