package reel

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type flat struct{ n int }

func (f flat) Frames() int { return f.n }
func (f flat) Frame(i int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 108, 192))
	for p := 0; p < len(img.Pix); p += 4 {
		img.Pix[p], img.Pix[p+3] = uint8(i*4), 255
	}
	return img
}

func TestEncode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	out := filepath.Join(t.TempDir(), "r.mp4")
	secs, err := Encode(flat{60}, 30, out)
	if err != nil {
		t.Fatal(err)
	}
	if secs != 2 {
		t.Errorf("length %.2f, want 2", secs)
	}
	if info, err := os.Stat(out); err != nil || info.Size() < 500 {
		t.Errorf("no usable video written: %v", err)
	}
	if _, err := Encode(flat{5}, 30, out); err == nil {
		t.Error("a clip under a second should be refused")
	}
}
