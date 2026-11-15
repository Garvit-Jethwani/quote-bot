package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/fogleman/gg"
)

// Theme is the colour set for one kind of post.
type Theme struct{ BG, FG, Accent, Muted color.RGBA }

func theme(bg, fg, accent, muted string) Theme {
	return Theme{Hex(bg), Hex(fg), Hex(accent), Hex(muted)}
}

// Themes maps a post kind to its colours.
var Themes = map[string]Theme{
	"film":    theme("#0F0F11", "#EFEAE0", "#E0A458", "#8D8A84"),
	"series":  theme("#0E1014", "#ECEAE4", "#8FB3E8", "#868B96"),
	"book":    theme("#F1EADB", "#1E1B17", "#A8472A", "#7B7266"),
	"music":   theme("#15171C", "#ECE9E2", "#7FB8A4", "#8A8F9C"),
	"art":     theme("#F5F3EE", "#17171A", "#2F4E7A", "#807C74"),
	"comedy":  theme("#F2C230", "#1A1712", "#A3271B", "#6E5A14"),
	"take":    theme("#B3261E", "#FFF6E9", "#FFD166", "#F0B8AE"),
	"thought": theme("#1F1535", "#F4EEFF", "#C6A6FF", "#8E7FB0"),
	"quote":   theme("#14213D", "#F7F3E9", "#FCA311", "#9AA5BD"),
}

// ThemeFor returns the theme for a kind, defaulting to the film theme.
func ThemeFor(kind string) Theme {
	if t, ok := Themes[kind]; ok {
		return t
	}
	return Themes["film"]
}

func canvas(th Theme) *gg.Context {
	dc := gg.NewContext(W, H)
	dc.SetColor(th.BG)
	dc.Clear()
	return dc
}

// chrome draws the header (series label, rule, page count) and the footer
// (swipe hint, handle). It returns the footer's y position.
func chrome(dc *gg.Context, th Theme, label string, n, total int, handle, hint string) float64 {
	f := face(SansBold, 24)
	end := tracked(dc, f, label, M, 78, 6, th.Accent)
	if total > 1 {
		count := fmt.Sprintf("%02d / %02d", n, total)
		cw := trackedWidth(f, count, 3)
		tracked(dc, f, count, W-M-cw, 78, 3, th.Muted)
		hline(dc, end+22, W-M-cw-22, 93, 1, th.Muted)
	} else { // a single image: no page count, nothing to swipe
		hint = ""
		hline(dc, end+22, W-M, 93, 1, th.Muted)
	}
	fy := float64(H - 104)
	if hint != "" {
		e := tracked(dc, f, hint, M, fy, 6, th.Accent)
		hline(dc, e+14, e+74, fy+15, 3, th.Accent)
		dc.SetColor(th.Accent)
		dc.MoveTo(e+74, fy+8)
		dc.LineTo(e+86, fy+15)
		dc.LineTo(e+74, fy+22)
		dc.ClosePath()
		dc.Fill()
	} else {
		hline(dc, M, M+60, fy+15, 3, th.Accent)
	}
	if handle != "" {
		hf := face(SansMedium, 24)
		tracked(dc, hf, handle, W-M-trackedWidth(hf, handle, 2), fy, 2, th.Muted)
	}
	return fy
}

func byline(p content.Post) string {
	who := p.Maker()
	if who == "" {
		return fmt.Sprint(p.Year)
	}
	return strings.ToUpper(fmt.Sprintf("%s · %d", who, p.Year))
}

// landscape reports whether an image is wide enough to crop to a 16:9 band.
func landscape(img image.Image) bool {
	b := img.Bounds()
	return float64(b.Dx())/float64(b.Dy()) >= 1.5
}

// Cover draws the first slide: the image (if any), the hook, and the title.
func Cover(p content.Post, th Theme, still image.Image, total int, cfg content.Config) image.Image {
	dc := canvas(th)
	fy := chrome(dc, th, p.Label, 1, total, cfg.Handle, cfg.SwipeHint)
	width := float64(W - 2*M)
	titleF := face(SerifItalic, 42)
	titleLines := wrap(titleF, p.Title, width)
	metaH := 3 + 36 + float64(len(titleLines))*54 + 12 + 30

	var hookF = face(Serif, 76)
	var hook []string
	var lh, y, gap float64
	if still != nil {
		if p.Kind == "art" || !landscape(still) {
			// paintings, posters and covers are shown whole on a panel, never cropped
			dc.SetColor(mix(th.BG, th.FG, 0.09))
			dc.DrawRectangle(0, 150, W, 608)
			dc.Fill()
			b := still.Bounds()
			fitted := contain(still, minInt(W-80, b.Dx()*2), minInt(608-56, b.Dy()*2))
			fb := fitted.Bounds()
			dc.DrawImage(fitted, (W-fb.Dx())/2, 150+(608-fb.Dy())/2)
		} else {
			dc.DrawImage(cover(still, W, 608, 0.5, 0.5), 0, 150)
		}
		top, bottom := 150.0+608+64, fy-56
		hookF, hook, lh = fit(Serif, p.Hook, width, bottom-top-metaH-40, 76, 40, 1.2)
		y, gap = top, 40
	} else { // typographic cover
		top, bottom := 190.0, fy-56
		hookF, hook, lh = fit(Serif, p.Hook, width, bottom-top-metaH-60, 112, 48, 1.16)
		y = top + math.Floor((bottom-top-(float64(len(hook))*lh+60+metaH))/2)
		gap = 60
	}
	y = para(dc, hookF, hook, M, y, lh, th.FG) + gap
	hline(dc, M, M+84, y, 3, th.Accent)
	y = para(dc, titleF, titleLines, M, y+36, 54, th.FG) + 12
	tracked(dc, face(SansMedium, 25), byline(p), M, y, 4, th.Muted)
	return dc.Image()
}

// Body draws a text slide: small label, heading, body, with an optional
// image band on top.
func Body(p content.Post, th Theme, s content.Slide, n, total int, cfg content.Config, still image.Image) image.Image {
	dc := canvas(th)
	fy := chrome(dc, th, p.Label, n, total, cfg.Handle, "")
	width := float64(W - 2*M)
	top := 190.0
	headSize, bodyHi := 70.0, 48
	if still != nil {
		dc.DrawImage(cover(still, W, 420, 0.5, 0.35), 0, 150)
		top = 150 + 420 + 64
		headSize, bodyHi = 56, 40
	}
	bottom := fy - 60
	headF := face(Serif, headSize)
	head := wrap(headF, s.Head, width)
	headLH := math.Floor(headSize * 1.18)
	fixed := 30 + 40 + float64(len(head))*headLH + 48
	bodyF, body, lh := fit(Sans, s.Body, width, bottom-top-fixed, bodyHi, 28, 1.5)
	y := top
	if still == nil {
		y = top + math.Max(0, math.Floor((bottom-top-fixed-float64(len(body))*lh)*2/5))
	}
	tracked(dc, face(SansBold, 24), fmt.Sprintf("%02d  %s", n-1, s.Kicker), M, y, 5, th.Accent)
	y = para(dc, headF, head, M, y+30+40, headLH, th.FG) + 48
	para(dc, bodyF, body, M, y, lh, th.FG)
	return dc.Image()
}

// QuoteSlide draws a quotation with a large quote mark and a source line.
func QuoteSlide(label string, th Theme, quote, by string, n, total int, cfg content.Config) image.Image {
	dc := canvas(th)
	fy := chrome(dc, th, label, n, total, cfg.Handle, "")
	width := float64(W - 2*M)
	byF := face(SansMedium, 25)
	var byLines []string
	if by != "" {
		byLines = wrap(byF, strings.ToUpper(by), width-40)
	}
	top, bottom := 190.0, fy-60
	byH := float64(len(byLines)) * 38
	qF, lines, lh := fit(SerifItalic, quote, width, bottom-top-150-100-byH, 78, 40, 1.3)
	block := 150 + float64(len(lines))*lh + 56 + 40 + byH
	y := top + math.Floor((bottom-top-block)/2)
	text(dc, face(SerifBold, 300), "“", M-8, y-70, th.Accent)
	y = para(dc, qF, lines, M, y+150, lh, th.FG) + 56
	hline(dc, M, M+84, y, 3, th.Accent)
	y += 40
	for _, l := range byLines {
		tracked(dc, byF, l, M, y, 4, th.Muted)
		y += 38
	}
	return dc.Image()
}

// Post draws every slide of a post. stills are optional images for it: the
// first goes on the cover, the second on the first text slide.
func Post(p content.Post, cfg content.Config, stills []image.Image) []image.Image {
	th := ThemeFor(p.Kind)
	if p.Quote != "" { // a one-liner: a single card
		by := p.Title // a comedian's name; the account's own takes are unsigned
		if p.Kind != "comedy" {
			by = ""
		}
		return []image.Image{QuoteSlide(p.Label, th, p.Quote, by, 1, 1, cfg)}
	}
	if len(stills) > 0 {
		switch p.Kind {
		case "art":
			if stills[0].Bounds().Dx() >= 1200 { // second slide gets a close-up
				fx, fy := 0.5, 0.5
				if len(p.Detail) == 2 {
					fx, fy = p.Detail[0], p.Detail[1]
				}
				stills = append(stills[:1:1], Detail(stills[0], fx, fy, W, 420, 2.3))
			}
		case "film", "series":
			th.Accent = Accent(stills[0], th.Accent)
			if len(stills) > 1 && !landscape(stills[1]) {
				stills = stills[:1]
			}
		}
	}
	total := p.SlideCount(cfg)
	var first image.Image
	if len(stills) > 0 {
		first = stills[0]
	}
	out := []image.Image{Cover(p, th, first, total, cfg)}
	for i, s := range p.Slides {
		n := i + 2
		if s.IsQuote() {
			out = append(out, QuoteSlide(p.Label, th, s.Quote, s.By, n, total, cfg))
			continue
		}
		var band image.Image
		if i == 0 && len(stills) > 1 {
			band = stills[1]
		}
		out = append(out, Body(p, th, s, n, total, cfg, band))
	}
	if line := p.Outro(cfg); line != "" {
		out = append(out, Outro(p, th, line, total, cfg))
	}
	return out
}

// Outro draws the closing slide: one line asking for a send, a save or a reply.
func Outro(p content.Post, th Theme, line string, total int, cfg content.Config) image.Image {
	dc := canvas(th)
	fy := chrome(dc, th, p.Label, total, total, cfg.Handle, "")
	width := float64(W - 2*M)
	top, bottom := 190.0, fy-60
	f, lines, lh := fit(Serif, line, width, bottom-top-160, 92, 48, 1.18)
	block := float64(len(lines))*lh + 56 + 34
	y := top + math.Floor((bottom-top-block)/2)
	y = para(dc, f, lines, M, y, lh, th.FG) + 56
	hline(dc, M, M+84, y-20, 3, th.Accent)
	if cfg.Handle != "" {
		tracked(dc, face(SansBold, 24), strings.ToUpper("More like this: "+cfg.Handle), M, y+16, 5, th.Muted)
	}
	return dc.Image()
}

// quotePalettes are the colour sets for fallback quote cards.
var quotePalettes = []Theme{
	theme("#14213D", "#F7F3E9", "#FCA311", "#9AA5BD"),
	theme("#F4EDE1", "#1F1B16", "#B5532B", "#7A7166"),
	theme("#1E3A2F", "#F1EBDD", "#D9A441", "#93AB9E"),
	theme("#4A1525", "#F6E9E0", "#E0A458", "#C098A2"),
	theme("#1B1B1E", "#EDEDED", "#E4572E", "#8C8C92"),
	theme("#E9DCC9", "#2B2A28", "#2F5D62", "#7D7468"),
}

// QuoteCard draws a fallback quote from quotes.json.
func QuoteCard(q content.Quote, cfg content.Config) image.Image {
	sum := 0
	for _, r := range q.Text {
		sum += int(r)
	}
	label := "FROM THE PAGE"
	by := q.By + ", " + q.Source
	if q.Type == "movie" {
		label = "FROM THE SCREEN"
		by = q.Source + ", " + q.By
	}
	return QuoteSlide(label, quotePalettes[sum%len(quotePalettes)], q.Text, by, 1, 1, cfg)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
