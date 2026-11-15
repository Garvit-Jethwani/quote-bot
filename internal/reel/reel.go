// Package reel turns a post's slides into a vertical video using ffmpeg.
package reel

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
)

// Video frame size (9:16) and the crossfade between slides, in seconds.
const (
	Width  = 1080
	Height = 1920
	fade   = 0.4
)

// Durations returns how long each slide stays on screen: long enough to read
// it at an unhurried pace.
func Durations(p content.Post, total int) []float64 {
	out := []float64{3.0} // the cover
	for _, s := range p.Slides {
		words := len(strings.Fields(s.Head + " " + s.Body + " " + s.Quote + " " + s.Text))
		d := float64(words)/3.0 + 1.0
		if d < 4 {
			d = 4
		}
		if d > 9 {
			d = 9
		}
		out = append(out, d)
	}
	for len(out) < total { // the closing slide
		out = append(out, 3.5)
	}
	return out
}

// Frame centres a slide on a 9:16 canvas of the same background colour, so
// there are no visible bars.
func Frame(slide image.Image, bg color.Color) *image.RGBA {
	frame := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(frame, frame.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	b := slide.Bounds()
	at := image.Pt((Width-b.Dx())/2, (Height-b.Dy())/2)
	draw.Draw(frame, image.Rectangle{at, at.Add(b.Size())}, slide, b.Min, draw.Src)
	return frame
}

// Build writes an MP4 to out: each slide in turn with a crossfade, a thin
// progress bar in the accent colour, and a silent audio track (some players
// refuse video with none). It returns the video's length in seconds.
func Build(slides []image.Image, secs []float64, bg, accent color.RGBA, out string) (float64, error) {
	if len(slides) < 2 || len(slides) != len(secs) {
		return 0, fmt.Errorf("a Reel needs at least two slides with a duration each (got %d slides, %d durations)", len(slides), len(secs))
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return 0, fmt.Errorf("ffmpeg is not installed: %w", err)
	}
	tmp, err := os.MkdirTemp("", "reel")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)

	total := 0.0
	args := []string{"-y", "-loglevel", "error"}
	for i, s := range slides {
		path := filepath.Join(tmp, fmt.Sprintf("%02d.png", i))
		f, err := os.Create(path)
		if err != nil {
			return 0, err
		}
		err = png.Encode(f, Frame(s, bg))
		f.Close()
		if err != nil {
			return 0, err
		}
		args = append(args, "-loop", "1", "-t", fmt.Sprintf("%.2f", secs[i]+fade), "-r", "30", "-i", path)
		total += secs[i]
	}
	length := fmt.Sprintf("%.2f", total+fade)
	bar := len(slides)
	audio := bar + 1
	args = append(args,
		"-f", "lavfi", "-t", length, "-i", fmt.Sprintf("color=c=0x%02X%02X%02X:s=%dx6:r=30", accent.R, accent.G, accent.B, Width),
		"-f", "lavfi", "-t", length, "-i", "anullsrc=r=44100:cl=stereo")

	var chain []string
	last, offset := "[0:v]", 0.0
	for i := 1; i < len(slides); i++ {
		offset += secs[i-1]
		next := fmt.Sprintf("[v%d]", i)
		chain = append(chain, fmt.Sprintf("%s[%d:v]xfade=transition=fade:duration=%.1f:offset=%.2f%s", last, i, fade, offset, next))
		last = next
	}
	barY := (Height-1350)/2 - 6 // just above the slide
	chain = append(chain,
		fmt.Sprintf("%s[%d:v]overlay=x='-w+w*t/%.2f':y=%d:shortest=1[p]", last, bar, total+fade, barY),
		"[p]format=yuv420p[v]")
	args = append(args,
		"-filter_complex", strings.Join(chain, ";"), "-map", "[v]", "-map", fmt.Sprintf("%d:a", audio),
		"-c:v", "libx264", "-preset", "medium", "-crf", "20", "-r", "30",
		"-c:a", "aac", "-b:a", "128k", "-shortest", "-movflags", "+faststart", out)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return 0, err
	}
	if msg, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(string(msg)))
	}
	return total + fade, nil
}
