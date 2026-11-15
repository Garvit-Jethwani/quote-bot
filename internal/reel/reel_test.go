package reel

import (
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

func TestDurations(t *testing.T) {
	p := content.Post{Slides: []content.Slide{
		{Head: "Short", Body: "A few words."},
		{Head: "Long", Body: strings.Repeat("word ", 200)},
	}}
	d := Durations(p, 4)
	if len(d) != 4 || d[0] != 3 || d[1] != 4 || d[2] != 9 || d[3] != 3.5 {
		t.Errorf("got %v", d)
	}
}

func TestBuild(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	slide := image.NewRGBA(image.Rect(0, 0, 1080, 1350))
	out := filepath.Join(t.TempDir(), "r.mp4")
	secs, err := Build([]image.Image{slide, slide}, []float64{1, 1}, color.RGBA{10, 10, 10, 255}, color.RGBA{250, 200, 0, 255}, out)
	if err != nil {
		t.Fatal(err)
	}
	if secs < 2 || secs > 3 {
		t.Errorf("length %.2f", secs)
	}
	if info, err := os.Stat(out); err != nil || info.Size() < 1000 {
		t.Errorf("no usable video written: %v", err)
	}
	if _, err := Build([]image.Image{slide}, []float64{1}, color.RGBA{}, color.RGBA{}, out); err == nil {
		t.Error("a single slide should be refused")
	}
}
