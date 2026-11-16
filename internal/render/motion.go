package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/fogleman/gg"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
)

// Reel frame size (9:16), frame rate, and the text column. Text stays inside
// the area Instagram leaves clear of its own buttons and caption.
const (
	RW      = 1080
	RH      = 1920
	FPS     = 30
	reelX   = 90.0
	reelW   = 880.0
	reelTop = 260.0
	reelEnd = 1480.0
	bgScale = 1.12 // backgrounds are drawn larger than the frame so they can drift
	bgW     = 1210
	bgH     = 2150
)

// palette is the colour set for one scene.
type palette struct{ fg, accent, muted color.RGBA }

// scene is one beat of a Reel: a background that slowly zooms and a function
// that draws the text for a given moment.
type scene struct {
	dur  float64
	bg   *image.RGBA
	draw func(dc *gg.Context, t float64)
}

// Clip is a finished Reel, ready to be turned into frames.
type Clip struct {
	scenes []scene
	bar    palette
}

// Seconds is the Reel's length.
func (c *Clip) Seconds() float64 {
	total := 0.0
	for _, s := range c.scenes {
		total += s.dur
	}
	return total
}

// Frames is the number of frames in the Reel.
func (c *Clip) Frames() int { return int(math.Round(c.Seconds() * FPS)) }

// Frame draws frame i.
func (c *Clip) Frame(i int) *image.RGBA {
	t := float64(i) / FPS
	idx := 0
	for idx < len(c.scenes)-1 && t >= c.scenes[idx].dur {
		t -= c.scenes[idx].dur
		idx++
	}
	s := c.scenes[idx]
	frame := image.NewRGBA(image.Rect(0, 0, RW, RH))

	// A slow push-in: start on the whole background, end on its centre.
	b := s.bg.Bounds()
	zoom := 1 + (bgScale-1)*math.Min(t/s.dur, 1)
	w, h := float64(b.Dx())/zoom, float64(b.Dy())/zoom
	x, y := (float64(b.Dx())-w)/2, (float64(b.Dy())-h)/2
	src := image.Rect(int(x), int(y), int(x+w), int(y+h))
	draw.ApproxBiLinear.Scale(frame, frame.Bounds(), s.bg, src, draw.Src, nil)

	dc := gg.NewContextForRGBA(frame)
	s.draw(dc, t)

	// Progress bars along the top, one per scene, like a story.
	n := float64(len(c.scenes))
	gap := 10.0
	segW := (reelW - gap*(n-1)) / n
	for k := range c.scenes {
		sx := reelX + float64(k)*(segW+gap)
		dc.SetColor(withAlpha(c.bar.muted, 0.45))
		dc.DrawRectangle(sx, 170, segW, 6)
		dc.Fill()
		fill := 0.0
		if k < idx {
			fill = 1
		} else if k == idx {
			fill = math.Min(t/s.dur, 1)
		}
		dc.SetColor(c.bar.fg)
		dc.DrawRectangle(sx, 170, segW*fill, 6)
		dc.Fill()
	}

	if t < 0.25 { // each scene fades up from black
		dc.SetColor(color.NRGBA{0, 0, 0, uint8(255 * (1 - t/0.25))})
		dc.DrawRectangle(0, 0, RW, RH)
		dc.Fill()
	}
	return frame
}

func withAlpha(c color.RGBA, a float64) color.NRGBA {
	a = math.Max(0, math.Min(1, a))
	return color.NRGBA{c.R, c.G, c.B, uint8(a * 255)}
}

// appear gives a line's opacity and downward offset at time t, for a line
// that starts arriving at delay. Lines rise into place as they fade in.
func appear(t, delay float64) (alpha, dy float64) {
	a := math.Max(0, math.Min(1, (t-delay)/0.4))
	e := 1 - (1-a)*(1-a)
	return e, (1 - e) * 36
}

// rise draws lines one after another, each arriving slightly later.
func rise(dc *gg.Context, f font.Face, lines []string, x, y, lh float64, c color.RGBA, t, delay, stagger float64) float64 {
	for i, l := range lines {
		a, dy := appear(t, delay+float64(i)*stagger)
		text(dc, f, l, x, y+dy, withAlpha(c, a))
		y += lh
	}
	return y
}

func trackedA(dc *gg.Context, f font.Face, s string, x, y, spacing float64, c color.RGBA, t, delay float64) {
	a, dy := appear(t, delay)
	tracked(dc, f, s, x, y+dy, spacing, withAlpha(c, a))
}

// stillBG turns an image into a full-frame background. dark is how much to
// dim it; blur softens it so text on top stays readable; shade adds a
// gradient that darkens the lower part, where the cover's text sits.
func stillBG(img image.Image, dark float64, blur, shade bool) *image.RGBA {
	w, h := bgW, bgH
	bg := toRGBA(cover(img, w, h, 0.5, 0.4))
	if blur { // shrink, smooth, and enlarge again: a cheap, soft blur
		small := image.NewRGBA(image.Rect(0, 0, w/16, h/16))
		draw.BiLinear.Scale(small, small.Bounds(), bg, bg.Bounds(), draw.Src, nil)
		for pass := 0; pass < 3; pass++ {
			small = boxBlur(small)
		}
		draw.BiLinear.Scale(bg, bg.Bounds(), small, small.Bounds(), draw.Src, nil)
	}
	for y := 0; y < h; y++ {
		k := 1 - dark
		if shade {
			if f := float64(y)/float64(h) - 0.3; f > 0 {
				k *= 1 - math.Min(1, f/0.45)*0.78
			}
		}
		row := bg.Pix[y*bg.Stride : y*bg.Stride+w*4]
		for i := 0; i < len(row); i += 4 {
			row[i] = uint8(float64(row[i]) * k)
			row[i+1] = uint8(float64(row[i+1]) * k)
			row[i+2] = uint8(float64(row[i+2]) * k)
		}
	}
	return bg
}

// plainBG is the background for posts with no picture: the theme colour
// drifting towards its accent.
func plainBG(th Theme) *image.RGBA {
	w, h := bgW, bgH
	bg := image.NewRGBA(image.Rect(0, 0, w, h))
	end := mix(th.BG, th.Accent, 0.2)
	for y := 0; y < h; y++ {
		c := mix(th.BG, end, float64(y)/float64(h))
		row := bg.Pix[y*bg.Stride : y*bg.Stride+w*4]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, 255
		}
	}
	return bg
}

// boxBlur averages each pixel with its eight neighbours.
func boxBlur(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, n int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					xx, yy := x+dx, y+dy
					if xx < 0 || yy < 0 || xx >= w || yy >= h {
						continue
					}
					i := yy*src.Stride + xx*4
					r, g, bl, n = r+int(src.Pix[i]), g+int(src.Pix[i+1]), bl+int(src.Pix[i+2]), n+1
				}
			}
			i := y*dst.Stride + x*4
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	return dst
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

// firstSentence returns the opening sentence of a paragraph. A Reel has time
// for one line per beat, not a whole slide of text.
func firstSentence(s string) string {
	for i := 0; i < len(s)-1; i++ {
		if (s[i] == '.' || s[i] == '?' || s[i] == '!') && s[i+1] == ' ' && i > 20 {
			return s[:i+1]
		}
	}
	return s
}

// readTime is how long a beat with this many words stays on screen.
func readTime(words int) float64 {
	return math.Max(2.8, math.Min(8, 1.0+float64(words)/3.5))
}

// Motion builds a Reel from a post: a cover scene with the hook, one scene
// per slide, and a closing scene. stills are optional pictures for it.
func Motion(p content.Post, cfg content.Config, stills []image.Image) *Clip {
	th := ThemeFor(p.Kind)
	onImage := palette{Hex("#F5F1E8"), th.Accent, Hex("#C2BDB2")}
	onPlain := palette{th.FG, th.Accent, th.Muted}
	if len(stills) > 0 && (p.Kind == "film" || p.Kind == "series") {
		onImage.accent = Accent(stills[0], Hex("#E0A458"))
	}
	if len(stills) > 0 && th.BG.R > 128 { // light themes need a lighter accent over dark pictures
		onImage.accent = mix(th.Accent, Hex("#FFFFFF"), 0.45)
	}

	pal := onPlain
	plain := plainBG(th)
	backdrop := func(i int) *image.RGBA { return plain }
	if len(stills) > 0 {
		pal = onImage
		soft := make([]*image.RGBA, len(stills))
		for i, s := range stills {
			soft[i] = stillBG(s, 0.62, true, false)
		}
		backdrop = func(i int) *image.RGBA { return soft[i%len(soft)] }
	}
	clip := &Clip{bar: pal}

	// ---- cover
	fullBleed := len(stills) > 0 && landscape(stills[0]) && p.Kind != "art"
	coverBG := backdrop(0)
	var card image.Image
	if fullBleed {
		coverBG = stillBG(stills[0], 0.08, false, true)
	} else if len(stills) > 0 {
		b := stills[0].Bounds()
		card = contain(stills[0], minInt(820, b.Dx()*2), minInt(700, b.Dy()*2))
	}
	hookF, hook, hookLH := fit(Serif, p.Hook, reelW, 560, 104, 56, 1.14)
	titleF := face(SerifItalic, 46)
	titleLines := wrap(titleF, p.Title, reelW)
	by := byline(p)
	clip.scenes = append(clip.scenes, scene{dur: 3.0, bg: coverBG, draw: func(dc *gg.Context, t float64) {
		trackedA(dc, face(SansBold, 28), p.Label, reelX, reelTop, 7, pal.accent, t, 0.1)
		if card != nil {
			a, _ := appear(t, 0.1)
			cb := card.Bounds()
			dc.Push()
			dc.DrawImage(card, (RW-cb.Dx())/2, 340+(700-cb.Dy())/2)
			dc.Pop()
			if a < 1 { // fade the picture in by covering it less and less
				dc.SetColor(color.NRGBA{0, 0, 0, uint8(120 * (1 - a))})
				dc.DrawRectangle(0, 330, RW, 720)
				dc.Fill()
			}
		}
		block := float64(len(hook))*hookLH + 64 + float64(len(titleLines))*58 + 44
		y := reelEnd - block
		y = rise(dc, hookF, hook, reelX, y, hookLH, pal.fg, t, 0.3, 0.16)
		a, _ := appear(t, 0.9)
		hline(dc, reelX, reelX+90*a, y+34, 4, pal.accent)
		y = rise(dc, titleF, titleLines, reelX, y+64, 58, pal.fg, t, 1.0, 0.1)
		if by != "" {
			trackedA(dc, face(SansMedium, 26), by, reelX, y+10, 4, pal.muted, t, 1.15)
		}
	}})

	// ---- one scene per slide
	for i, s := range p.Slides {
		s := s
		n := i + 1
		bg := backdrop(i + 1)
		switch {
		case s.Text != "":
			f, lines, lh := fit(SerifMedium, s.Text, reelW, 900, 78, 44, 1.3)
			clip.scenes = append(clip.scenes, scene{dur: readTime(len(strings.Fields(s.Text))), bg: bg, draw: func(dc *gg.Context, t float64) {
				y := reelTop + math.Floor((reelEnd-reelTop-float64(len(lines))*lh)/2)
				rise(dc, f, lines, reelX, y, lh, pal.fg, t, 0.2, 0.12)
			}})
		case s.IsQuote():
			f, lines, lh := fit(SerifItalic, s.Quote, reelW, 700, 84, 46, 1.28)
			byF := face(SansMedium, 26)
			byLines := wrap(byF, strings.ToUpper(s.By), reelW)
			clip.scenes = append(clip.scenes, scene{dur: readTime(len(strings.Fields(s.Quote)) + 3), bg: bg, draw: func(dc *gg.Context, t float64) {
				block := 150 + float64(len(lines))*lh + 60 + float64(len(byLines))*40
				y := reelTop + math.Floor((reelEnd-reelTop-block)/2)
				a, dy := appear(t, 0.1)
				text(dc, face(SerifBold, 300), "“", reelX-8, y-70+dy, withAlpha(pal.accent, a))
				y = rise(dc, f, lines, reelX, y+150, lh, pal.fg, t, 0.3, 0.14) + 60
				for k, l := range byLines {
					trackedA(dc, byF, l, reelX, y+float64(k)*40, 4, pal.muted, t, 0.9)
				}
			}})
		default:
			line := firstSentence(s.Body)
			headF, head, headLH := fit(Serif, s.Head, reelW, 380, 92, 54, 1.14)
			lineF, lines, lineLH := fit(Sans, line, reelW, 460, 56, 36, 1.42)
			words := len(strings.Fields(s.Head + " " + line))
			kicker := fmt.Sprintf("%02d  %s", n, s.Kicker)
			clip.scenes = append(clip.scenes, scene{dur: readTime(words), bg: bg, draw: func(dc *gg.Context, t float64) {
				block := 70 + float64(len(head))*headLH + 96 + float64(len(lines))*lineLH
				y := reelTop + math.Floor((reelEnd-reelTop-block)/2)
				trackedA(dc, face(SansBold, 28), kicker, reelX, y, 6, pal.accent, t, 0.1)
				y = rise(dc, headF, head, reelX, y+70, headLH, pal.fg, t, 0.25, 0.14)
				a, _ := appear(t, 0.7)
				hline(dc, reelX, reelX+90*a, y+40, 4, pal.accent)
				rise(dc, lineF, lines, reelX, y+96, lineLH, pal.fg, t, 0.8, 0.12)
			}})
		}
	}

	// ---- closing scene
	last := p.Outro(cfg)
	if last == "" {
		last = "More like this on the page."
	}
	lastF, lastLines, lastLH := fit(Serif, last, reelW, 600, 92, 52, 1.16)
	clip.scenes = append(clip.scenes, scene{dur: 2.6, bg: backdrop(len(p.Slides) + 1), draw: func(dc *gg.Context, t float64) {
		block := float64(len(lastLines))*lastLH + 80
		y := reelTop + math.Floor((reelEnd-reelTop-block)/2)
		y = rise(dc, lastF, lastLines, reelX, y, lastLH, pal.fg, t, 0.2, 0.14)
		a, _ := appear(t, 0.7)
		hline(dc, reelX, reelX+90*a, y+36, 4, pal.accent)
		if cfg.Handle != "" {
			trackedA(dc, face(SansBold, 28), strings.ToUpper(cfg.Handle), reelX, y+72, 5, pal.muted, t, 0.8)
		}
	}})
	return clip
}
