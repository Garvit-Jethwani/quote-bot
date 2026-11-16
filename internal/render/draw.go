// Package render draws posts as 1080x1350 slides.
package render

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// Slide size (Instagram portrait, 4:5) and side margin.
const (
	W = 1080
	H = 1350
	M = 96
)

//go:embed fonts/*.ttf
var fontFiles embed.FS

// Font names available to the renderer.
const (
	Serif       = "PlayfairDisplay-SemiBold"
	SerifMedium = "PlayfairDisplay-Medium"
	SerifBold   = "PlayfairDisplay-Bold"
	SerifItalic = "PlayfairDisplay-MediumItalic"
	Sans        = "Inter-Regular"
	SansMedium  = "Inter-Medium"
	SansBold    = "Inter-SemiBold"
	SansHeavy   = "Inter-ExtraBold"
)

var (
	fontMu sync.Mutex
	fonts  = map[string]*opentype.Font{}
	faces  = map[string]font.Face{}
)

// face returns a cached font face at a pixel size.
func face(name string, size float64) font.Face {
	fontMu.Lock()
	defer fontMu.Unlock()
	key := fmt.Sprintf("%s@%g", name, size)
	if f, ok := faces[key]; ok {
		return f
	}
	parsed, ok := fonts[name]
	if !ok {
		data, err := fontFiles.ReadFile("fonts/" + name + ".ttf")
		if err != nil {
			panic(err) // fonts are embedded at build time; a miss is a programming error
		}
		if parsed, err = opentype.Parse(data); err != nil {
			panic(err)
		}
		fonts[name] = parsed
	}
	f, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	faces[key] = f
	return f
}

func textWidth(f font.Face, s string) float64 {
	return float64(font.MeasureString(f, s)) / 64
}

// text draws s with its top edge at y, the way the layout numbers assume.
func text(dc *gg.Context, f font.Face, s string, x, y float64, c color.Color) {
	dc.SetFontFace(f)
	dc.SetColor(c)
	dc.DrawString(s, x, y+float64(f.Metrics().Ascent)/64)
}

// tracked draws letter-spaced text and returns the x position after it.
func tracked(dc *gg.Context, f font.Face, s string, x, y, spacing float64, c color.Color) float64 {
	for _, r := range s {
		text(dc, f, string(r), x, y, c)
		x += textWidth(f, string(r)) + spacing
	}
	return x
}

func trackedWidth(f font.Face, s string, spacing float64) float64 {
	w := 0.0
	for _, r := range s {
		w += textWidth(f, string(r)) + spacing
	}
	return w - spacing
}

// wrap breaks text into lines no wider than width. Newlines are kept.
func wrap(f font.Face, s string, width float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			trial := strings.TrimSpace(line + " " + word)
			if line != "" && textWidth(f, trial) > width {
				out = append(out, line)
				line = word
			} else {
				line = trial
			}
		}
		out = append(out, line)
	}
	return out
}

// fit finds the largest size from hi down to lo at which the wrapped text
// fits in maxH. It returns the face, the lines and the line height.
func fit(name, s string, width, maxH float64, hi, lo int, ratio float64) (font.Face, []string, float64) {
	var f font.Face
	var lines []string
	var lh float64
	for size := hi; size >= lo; size -= 2 {
		f = face(name, float64(size))
		lines = wrap(f, s, width)
		lh = math.Floor(float64(size) * ratio)
		widest := 0.0
		for _, l := range lines {
			widest = math.Max(widest, textWidth(f, l))
		}
		if float64(len(lines))*lh <= maxH && widest <= width {
			break
		}
	}
	return f, lines, lh
}

// para draws lines and returns the y position after the last one.
func para(dc *gg.Context, f font.Face, lines []string, x, y, lh float64, c color.Color) float64 {
	for _, l := range lines {
		text(dc, f, l, x, y, c)
		y += lh
	}
	return y
}

// hline draws a horizontal rule centred on y.
func hline(dc *gg.Context, x1, x2, y, width float64, c color.Color) {
	dc.SetColor(c)
	dc.DrawRectangle(x1, y-width/2, x2-x1, width)
	dc.Fill()
}

// Hex parses "#RRGGBB".
func Hex(s string) color.RGBA {
	var r, g, b uint8
	fmt.Sscanf(strings.TrimPrefix(s, "#"), "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

// cover scales and crops src to fill w x h, keeping the point (cx, cy),
// given as fractions, as close to the centre as the crop allows.
func cover(src image.Image, w, h int, cx, cy float64) image.Image {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	cw, ch := sw, sw*float64(h)/float64(w)
	if ch > sh {
		ch, cw = sh, sh*float64(w)/float64(h)
	}
	x := (sw - cw) * cx
	y := (sh - ch) * cy
	r := image.Rect(b.Min.X+int(x), b.Min.Y+int(y), b.Min.X+int(x+cw), b.Min.Y+int(y+ch))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, r, draw.Src, nil)
	return dst
}

// contain scales src to fit inside maxW x maxH without cropping.
func contain(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	scale := math.Min(float64(maxW)/float64(b.Dx()), float64(maxH)/float64(b.Dy()))
	w, h := int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

// Detail crops a close-up of src around the point (fx, fy), given as
// fractions, shaped w:h.
func Detail(src image.Image, fx, fy float64, w, h int, zoom float64) image.Image {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	cw := sw / zoom
	ch := cw * float64(h) / float64(w)
	if ch > sh {
		ch, cw = sh, sh*float64(w)/float64(h)
	}
	x := math.Min(math.Max(fx*sw-cw/2, 0), sw-cw)
	y := math.Min(math.Max(fy*sh-ch/2, 0), sh-ch)
	r := image.Rect(b.Min.X+int(x), b.Min.Y+int(y), b.Min.X+int(x+cw), b.Min.Y+int(y+ch))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, r, draw.Src, nil)
	return dst
}

// Accent picks a readable accent colour from an image's dominant hue, or
// returns fallback when the image has too little colour.
func Accent(src image.Image, fallback color.RGBA) color.RGBA {
	small := image.NewRGBA(image.Rect(0, 0, 48, 27))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), src, src.Bounds(), draw.Src, nil)
	bins := map[int][]float64{}
	for i := 0; i < len(small.Pix); i += 4 {
		h, s, v := rgbToHSV(small.Pix[i], small.Pix[i+1], small.Pix[i+2])
		if s > 0.3 && v > 0.25 && v < 0.95 {
			bins[int(h*12)] = append(bins[int(h*12)], h)
		}
	}
	var best []float64
	for _, hues := range bins {
		if len(hues) > len(best) {
			best = hues
		}
	}
	if len(best) < 40 {
		return fallback
	}
	sum := 0.0
	for _, h := range best {
		sum += h
	}
	return hsvToRGB(sum/float64(len(best)), 0.5, 0.93)
}

func rgbToHSV(r8, g8, b8 uint8) (h, s, v float64) {
	r, g, b := float64(r8)/255, float64(g8)/255, float64(b8)/255
	max, min := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	v = max
	d := max - min
	if max == 0 || d == 0 {
		return 0, 0, v
	}
	s = d / max
	switch max {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	if h < 0 {
		h++
	}
	return h, s, v
}

func hsvToRGB(h, s, v float64) color.RGBA {
	i := math.Floor(h * 6)
	f := h*6 - i
	p, q, t := v*(1-s), v*(1-f*s), v*(1-(1-f)*s)
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return color.RGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
}

// SaveJPEG writes img to path, creating the folder if needed.
func SaveJPEG(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 92})
}
