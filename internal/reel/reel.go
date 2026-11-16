// Package reel encodes a sequence of frames as a vertical video using ffmpeg.
package reel

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source produces the frames of a video.
type Source interface {
	Frames() int
	Frame(i int) *image.RGBA
}

// Encode writes src to out as an H.264 MP4 at the given frame rate, with a
// silent audio track (some players refuse video with none, and it gives
// Instagram a track to replace when music is added in the app). It returns
// the video's length in seconds.
func Encode(src Source, fps int, out string) (float64, error) {
	n := src.Frames()
	if n < fps {
		return 0, fmt.Errorf("a Reel needs at least a second of video (got %d frames)", n)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return 0, fmt.Errorf("ffmpeg is not installed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return 0, err
	}
	first := src.Frame(0)
	size := fmt.Sprintf("%dx%d", first.Bounds().Dx(), first.Bounds().Dy())
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "rawvideo", "-pix_fmt", "rgba", "-s", size, "-r", fmt.Sprint(fps), "-i", "-",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
		"-shortest", "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	var writeErr error
	for i := 0; i < n && writeErr == nil; i++ {
		frame := first
		if i > 0 {
			frame = src.Frame(i)
		}
		_, writeErr = stdin.Write(frame.Pix)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		return 0, fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if writeErr != nil {
		return 0, writeErr
	}
	return float64(n) / float64(fps), nil
}
