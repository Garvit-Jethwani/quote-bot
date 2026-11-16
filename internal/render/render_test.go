package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

func solid(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	return img
}

func TestEveryPostInTheQueueDraws(t *testing.T) {
	posts, err := content.LoadPosts("../../posts.json")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := content.LoadConfig("../../config.json")
	for _, p := range posts {
		slides := Post(p, cfg, nil)
		if len(slides) != p.SlideCount(cfg) {
			t.Errorf("%s: drew %d slides, want %d", p.ID, len(slides), p.SlideCount(cfg))
		}
		for _, s := range slides {
			if b := s.Bounds(); b.Dx() != W || b.Dy() != H {
				t.Errorf("%s: slide is %dx%d", p.ID, b.Dx(), b.Dy())
			}
		}
	}
}

func TestPostWithImages(t *testing.T) {
	cfg := content.Config{Handle: "@x", SwipeHint: "SWIPE"}
	slides := []content.Slide{{Kicker: "K", Head: "H", Body: "B"}, {Kicker: "K", Head: "H", Body: "B"}, {Quote: "Q", By: "who"}}
	still := solid(1920, 1080, color.RGBA{200, 60, 40, 255})
	poster := solid(1000, 1500, color.RGBA{40, 60, 200, 255})

	film := content.Post{ID: "f", Kind: "film", Label: "SCENE STUDY", Title: "T", Year: 2000, Director: "D", Hook: "Hook", Slides: slides}
	out := Post(film, cfg, []image.Image{still, still})
	if len(out) != 4 {
		t.Fatalf("got %d slides", len(out))
	}
	// The still should fill the 16:9 band on the cover.
	if r, _, _, _ := out[0].At(W/2, 400).RGBA(); r>>8 < 150 {
		t.Error("cover does not show the still")
	}
	// A poster is shown whole on a panel, so the band's edge is panel, not poster.
	out = Post(film, cfg, []image.Image{poster})
	if _, _, b, _ := out[0].At(20, 400).RGBA(); b>>8 > 150 {
		t.Error("a portrait poster was cropped to fill the band")
	}
	if _, _, b, _ := out[0].At(W/2, 400).RGBA(); b>>8 < 150 {
		t.Error("cover does not show the poster")
	}

	art := content.Post{ID: "a", Kind: "art", Label: "LOOK CLOSER", Title: "T", Year: 1880, Artist: "A B", Hook: "Hook", Slides: slides, Detail: []float64{0.5, 0.5}}
	painting := solid(1686, 1290, color.RGBA{30, 160, 90, 255})
	out = Post(art, cfg, []image.Image{painting})
	if _, g, _, _ := out[1].At(W/2, 300).RGBA(); g>>8 < 120 {
		t.Error("the first text slide of an art post should show a close-up")
	}
}

func TestWrapAndFit(t *testing.T) {
	f := face(Sans, 40)
	lines := wrap(f, "one two three four five six seven eight nine ten eleven twelve", 400)
	if len(lines) < 2 {
		t.Fatal("long text should wrap")
	}
	for _, l := range lines {
		if textWidth(f, l) > 400 {
			t.Errorf("line %q is wider than the limit", l)
		}
	}
	if got := wrap(f, "a\nb", 400); len(got) != 2 {
		t.Errorf("newlines should be kept, got %v", got)
	}
	_, _, lh := fit(Sans, "a very long piece of text that will not fit at the largest size in so small a box", 300, 200, 80, 20, 1.5)
	if lh >= 80*1.5 {
		t.Error("fit should shrink text that does not fit")
	}
}

func TestAccent(t *testing.T) {
	grey := color.RGBA{1, 2, 3, 255}
	if got := Accent(solid(100, 100, color.RGBA{120, 120, 120, 255}), grey); got != grey {
		t.Error("a colourless image should keep the fallback accent")
	}
	if got := Accent(solid(100, 100, color.RGBA{200, 40, 40, 255}), grey); got.R < got.B {
		t.Errorf("a red image should give a warm accent, got %v", got)
	}
}

func TestMood(t *testing.T) {
	cfg := content.Config{Handle: "@x"}
	wide := solid(1920, 1080, color.RGBA{60, 90, 120, 255})
	for _, c := range []struct {
		p     content.Post
		still image.Image
	}{
		{content.Post{Label: "MOOD", Kind: "mood", Doodle: "guy-floor", Quote: "me after one email"}, nil},
		{content.Post{Label: "MOOD", Kind: "film", Quote: "a quiet line over a still"}, wide},
		{content.Post{Label: "MOOD", Kind: "art", Quote: "a line above a painting"}, wide},
		{content.Post{Label: "MOOD", Kind: "art", Quote: "no picture was found"}, nil},
	} {
		var stills []image.Image
		if c.still != nil {
			stills = []image.Image{c.still}
		}
		out := Post(c.p, cfg, stills)
		if len(out) != 1 {
			t.Fatalf("%s mood drew %d images, want 1", c.p.Kind, len(out))
		}
		if b := out[0].Bounds(); b.Dx() != W || b.Dy() != H {
			t.Errorf("%s mood is %dx%d", c.p.Kind, b.Dx(), b.Dy())
		}
	}
}
