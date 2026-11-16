package render

import (
	"os"
	"testing"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/fogleman/gg"
)

func TestDoodlesMatchContent(t *testing.T) {
	names := DoodleNames()
	if len(names) != len(content.Doodles) {
		t.Errorf("render has %d doodles, content lists %d", len(names), len(content.Doodles))
	}
	dc := gg.NewContext(200, 200)
	for _, n := range names {
		if !content.Doodles[n] {
			t.Errorf("doodle %q is not listed in content.Doodles", n)
		}
		for _, progress := range []float64{0, 0.3, 1} {
			if !Doodle(dc, n, 0, 0, 200, Hex("#000000"), Hex("#FF0000"), Hex("#FFFFFF"), progress, 1) {
				t.Errorf("doodle %q did not draw", n)
			}
		}
	}
	if Doodle(dc, "nope", 0, 0, 200, Hex("#000000"), Hex("#FF0000"), Hex("#FFFFFF"), 1, 0) {
		t.Error("an unknown doodle should report false")
	}
}

func TestDoodleSheet(t *testing.T) {
	out := os.Getenv("DOODLE_SHEET")
	if out == "" {
		t.Skip()
	}
	names := DoodleNames()
	th := Themes["bit"]
	const cell, cols = 300, 6
	rows := (len(names) + cols - 1) / cols
	dc := gg.NewContext(cols*cell, rows*cell)
	dc.SetColor(th.BG)
	dc.Clear()
	for i, n := range names {
		x, y := float64(i%cols*cell), float64(i/cols*cell)
		Doodle(dc, n, x+40, y+30, 220, th.FG, th.Accent, th.BG, 1, 0)
		text(dc, face(Sans, 18), n, x+40, y+268, th.Muted)
	}
	SaveJPEG(dc.Image(), out)
}
