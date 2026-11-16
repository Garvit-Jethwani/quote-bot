package render

import (
	"image/color"
	"math"
	"sort"

	"github.com/Garvit-Jethwani/quote-bot/internal/content"
	"github.com/fogleman/gg"
)

// Doodles are small drawings made of wobbly pen strokes, so they look drawn
// by hand rather than set by a machine. Each one is described on a 100 by 100
// grid as a list of strokes. Because a doodle is only strokes, it can be
// drawn partly (for a draw-on animation) and redrawn with a different wobble
// (so it shimmers a little in video, the way hand-drawn animation does).

type dpt struct{ x, y float64 }

type fillKind int

const (
	noFill fillKind = iota
	accentFill
	inkFill
	paperFill
)

type dstroke struct {
	pts    []dpt
	fill   fillKind
	accent bool // draw the line itself in the accent colour
}

// ---- building blocks

func pl(xy ...float64) []dpt {
	out := make([]dpt, 0, len(xy)/2)
	for i := 0; i+1 < len(xy); i += 2 {
		out = append(out, dpt{xy[i], xy[i+1]})
	}
	return out
}

// arc traces part of an ellipse from angle a0 to a1, in degrees. Angles grow
// clockwise on screen and 270 is the top.
func arc(cx, cy, rx, ry, a0, a1 float64) []dpt {
	n := int(math.Abs(a1-a0)/6) + 2
	out := make([]dpt, n+1)
	for i := 0; i <= n; i++ {
		a := (a0 + (a1-a0)*float64(i)/float64(n)) * math.Pi / 180
		out[i] = dpt{cx + rx*math.Cos(a), cy + ry*math.Sin(a)}
	}
	return out
}

func ring(cx, cy, rx, ry float64) []dpt { return arc(cx, cy, rx, ry, -100, 265) }

func quad(x0, y0, cx, cy, x1, y1 float64) []dpt {
	const n = 14
	out := make([]dpt, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		u := 1 - t
		out[i] = dpt{u*u*x0 + 2*u*t*cx + t*t*x1, u*u*y0 + 2*u*t*cy + t*t*y1}
	}
	return out
}

func cubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float64) []dpt {
	const n = 20
	out := make([]dpt, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		u := 1 - t
		a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		out[i] = dpt{a*x0 + b*c1x + c*c2x + d*x1, a*y0 + b*c1y + c*c2y + d*y1}
	}
	return out
}

func join(parts ...[]dpt) []dpt {
	var out []dpt
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func closed(p []dpt) []dpt { return append(append([]dpt{}, p...), p[0]) }

func mirror(p []dpt) []dpt {
	out := make([]dpt, len(p))
	for i, q := range p {
		out[i] = dpt{100 - q.x, q.y}
	}
	return out
}

func reverse(p []dpt) []dpt {
	out := make([]dpt, len(p))
	for i, q := range p {
		out[len(p)-1-i] = q
	}
	return out
}

func rotate(p []dpt, deg, cx, cy float64) []dpt {
	s, c := math.Sincos(deg * math.Pi / 180)
	out := make([]dpt, len(p))
	for i, q := range p {
		dx, dy := q.x-cx, q.y-cy
		out[i] = dpt{cx + dx*c - dy*s, cy + dx*s + dy*c}
	}
	return out
}

func star(cx, cy, r float64) []dpt {
	var out []dpt
	for i := 0; i <= 10; i++ {
		rr := r
		if i%2 == 1 {
			rr = r * 0.45
		}
		a := (-90 + float64(i)*36) * math.Pi / 180
		out = append(out, dpt{cx + rr*math.Cos(a), cy + rr*math.Sin(a)})
	}
	return out
}

func line(p []dpt) dstroke              { return dstroke{pts: p} }
func shape(p []dpt, f fillKind) dstroke { return dstroke{pts: closed(p), fill: f} }
func dot(cx, cy, r float64) dstroke     { return dstroke{pts: ring(cx, cy, r, r), fill: inkFill} }
func spin(s []dstroke, deg float64) []dstroke {
	for i := range s {
		s[i].pts = rotate(s[i].pts, deg, 50, 50)
	}
	return s
}

// ---- the drawings

var doodles = map[string]func() []dstroke{
	"ghost": func() []dstroke {
		body := join(pl(24, 88, 24, 46), arc(50, 46, 26, 30, 180, 360), pl(76, 88, 67, 79, 58, 88, 50, 79, 42, 88, 33, 79))
		return []dstroke{
			shape(body, accentFill),
			{pts: ring(41, 44, 3.2, 5), fill: inkFill},
			{pts: ring(59, 44, 3.2, 5), fill: inkFill},
			line(ring(50, 60, 4, 5.5)),
			line(quad(10, 52, 4, 44, 12, 36)),
			line(quad(90, 52, 96, 44, 88, 36)),
		}
	},
	"bat": func() []dstroke {
		wing := join(quad(45, 46, 28, 22, 4, 30), quad(4, 30, 17, 40, 15, 60), quad(15, 60, 23, 48, 30, 62), quad(30, 62, 37, 52, 45, 62))
		return []dstroke{
			shape(wing, accentFill),
			shape(mirror(wing), accentFill),
			shape(ring(50, 54, 7, 13), paperFill),
			shape(join(pl(44, 36, 43, 24, 48, 31), pl(52, 31, 57, 24, 56, 36), arc(50, 38, 7, 7, 20, 160)), paperFill),
			dot(47.5, 38, 1.2),
			dot(52.5, 38, 1.2),
			line(pl(48.5, 42.5, 50, 44, 51.5, 42.5)),
		}
	},
	"martini": func() []dstroke {
		return []dstroke{
			shape(pl(27, 30, 73, 30, 50, 56), accentFill),
			line(closed(pl(20, 22, 80, 22, 50, 56))),
			line(pl(50, 56, 50, 86)),
			line(quad(32, 89, 50, 83, 68, 89)),
			line(pl(76, 8, 53, 42)),
			shape(ring(61, 31, 5, 5), paperFill),
			dot(62, 30, 1.3),
			line(pl(86, 26, 92, 20)),
			line(pl(88, 34, 96, 33)),
		}
	},
	"ship": func() []dstroke {
		s := []dstroke{
			shape(pl(8, 58, 92, 58, 82, 76, 18, 76), accentFill),
			line(pl(22, 58, 22, 50, 76, 50, 76, 58)),
			shape(pl(30, 50, 32, 30, 39, 30, 40, 50), paperFill),
			shape(pl(45, 50, 47, 30, 54, 30, 55, 50), paperFill),
			shape(pl(60, 50, 62, 30, 69, 30, 70, 50), paperFill),
			line(quad(36, 26, 30, 16, 40, 10)),
			line(quad(51, 26, 45, 16, 55, 10)),
			line(quad(66, 26, 60, 16, 70, 10)),
			dot(30, 66, 1.3), dot(42, 66, 1.3), dot(54, 66, 1.3), dot(66, 66, 1.3),
			line(join(quad(4, 88, 12, 80, 20, 88), quad(20, 88, 28, 80, 36, 88), quad(36, 88, 44, 80, 52, 88), quad(52, 88, 60, 80, 68, 88), quad(68, 88, 76, 80, 84, 88), quad(84, 88, 92, 80, 98, 88))),
		}
		return s
	},
	"wizard": func() []dstroke {
		cone := join(quad(26, 76, 40, 44, 62, 8), quad(62, 8, 62, 44, 74, 76), quad(74, 76, 50, 84, 26, 76))
		return []dstroke{
			shape(cone, accentFill),
			line(join(arc(50, 78, 42, 9, 200, 340), arc(50, 78, 42, 9, -20, 200))),
			line(quad(31, 66, 50, 74, 71, 66)),
			shape(star(52, 46, 8), paperFill),
			line(star(88, 22, 5)),
			line(star(12, 36, 4)),
		}
	},
	"mug": func() []dstroke {
		return []dstroke{
			shape(join(pl(22, 34, 66, 34), quad(66, 34, 66, 84, 60, 86), pl(28, 86), quad(28, 86, 22, 84, 22, 34)), accentFill),
			line(arc(66, 58, 17, 15, -62, 62)),
			line(arc(66, 58, 9, 8, -58, 58)),
			shape(star(44, 60, 11), paperFill),
			line(cubic(34, 26, 28, 18, 40, 14, 34, 5)),
			line(cubic(50, 26, 44, 18, 56, 14, 50, 5)),
		}
	},
	"bone": func() []dstroke {
		left := join(arc(20, 42, 9, 9, 20, -250), arc(20, 58, 9, 9, 250, -20))
		return spin([]dstroke{
			shape(join(left, reverse(mirror(left))), accentFill),
			line(pl(34, 49, 46, 49)),
			line(pl(56, 52, 64, 52)),
		}, -28)
	},
	"fridge": func() []dstroke {
		return []dstroke{
			shape(pl(27, 8, 73, 8, 73, 90, 27, 90), accentFill),
			line(pl(27, 36, 73, 36)),
			line(pl(34, 18, 34, 30)),
			line(pl(34, 44, 34, 62)),
			line(pl(32, 90, 32, 95)),
			line(pl(68, 90, 68, 95)),
			shape(pl(52, 46, 64, 45, 65, 58, 53, 59), paperFill),
			line(pl(55, 50, 62, 49.5)),
			line(pl(55, 54, 61, 53.5)),
			line(pl(80, 30, 88, 26)),
			line(pl(81, 38, 90, 38)),
		}
	},
	"book": func() []dstroke {
		page := join(quad(50, 30, 32, 20, 10, 26), pl(10, 76), quad(10, 76, 32, 70, 50, 80))
		return []dstroke{
			shape(page, accentFill),
			shape(mirror(page), accentFill),
			line(pl(50, 30, 50, 80)),
			line(quad(18, 38, 30, 34, 42, 40)),
			line(quad(18, 48, 30, 44, 42, 50)),
			line(quad(18, 58, 30, 54, 42, 60)),
			line(quad(58, 40, 70, 34, 82, 38)),
			line(quad(58, 50, 70, 44, 82, 48)),
			shape(pl(64, 66, 72, 64, 72, 92, 68, 87, 64, 92), paperFill),
		}
	},
	"parrot": func() []dstroke {
		body := join(cubic(42, 30, 26, 44, 28, 66, 40, 78), pl(34, 97, 41, 92, 44, 98, 50, 80), cubic(50, 80, 70, 70, 72, 46, 64, 32))
		return []dstroke{
			shape(body, accentFill),
			shape(ring(54, 24, 14, 13), accentFill),
			shape(join(quad(64, 16, 80, 16, 76, 34), quad(76, 34, 70, 30, 66, 30)), paperFill),
			{pts: ring(57, 21, 2, 2), fill: inkFill},
			line(quad(40, 44, 58, 54, 46, 72)),
			line(quad(46, 10, 44, 2, 52, 4)),
			line(pl(12, 80, 88, 80)),
			line(pl(42, 78, 40, 84)),
			line(pl(50, 79, 52, 84)),
		}
	},
	"dumbbell": func() []dstroke {
		big := pl(12, 28, 23, 28, 23, 72, 12, 72)
		small := pl(23, 36, 30, 36, 30, 64, 23, 64)
		return spin([]dstroke{
			shape(big, accentFill),
			shape(mirror(big), accentFill),
			shape(small, paperFill),
			shape(mirror(small), paperFill),
			line(pl(30, 47, 70, 47)),
			line(pl(30, 53, 70, 53)),
		}, -22)
	},
	"van": func() []dstroke {
		return []dstroke{
			shape(pl(8, 34, 62, 34, 78, 52, 92, 56, 92, 76, 8, 76), accentFill),
			shape(pl(50, 40, 60, 40, 70, 52, 50, 52), paperFill),
			line(pl(46, 34, 46, 76)),
			line(pl(52, 60, 58, 60)),
			shape(ring(28, 78, 10, 10), paperFill),
			shape(ring(72, 78, 10, 10), paperFill),
			dot(28, 78, 2.5),
			dot(72, 78, 2.5),
			line(pl(14, 46, 38, 46)),
			line(pl(14, 54, 38, 54)),
			line(pl(2, 92, 98, 92)),
		}
	},
	"wifi": func() []dstroke {
		return []dstroke{
			dot(50, 80, 4.5),
			line(arc(50, 80, 17, 17, 222, 318)),
			{pts: arc(50, 80, 33, 33, 222, 318), accent: true},
			{pts: arc(50, 80, 49, 49, 222, 318), accent: true},
			line(pl(80, 62, 92, 74)),
			line(pl(92, 62, 80, 74)),
		}
	},
	"lift": func() []dstroke {
		return []dstroke{
			shape(pl(18, 20, 76, 20, 76, 92, 18, 92), accentFill),
			line(pl(47, 20, 47, 92)),
			shape(pl(36, 6, 58, 6, 58, 15, 36, 15), paperFill),
			line(pl(43, 12.5, 47, 8, 51, 12.5)),
			shape(pl(83, 46, 93, 46, 93, 68, 83, 68), paperFill),
			dot(88, 52, 1.8),
			dot(88, 61, 1.8),
			line(pl(10, 92, 96, 92)),
		}
	},
	"phone": func() []dstroke {
		return []dstroke{
			shape(join(quad(30, 12, 30, 6, 36, 6), pl(64, 6), quad(64, 6, 70, 6, 70, 12), pl(70, 88), quad(70, 88, 70, 94, 64, 94), pl(36, 94), quad(36, 94, 30, 94, 30, 88)), paperFill),
			shape(pl(35, 24, 56, 24, 56, 35, 39, 35, 35, 39), accentFill),
			shape(pl(44, 44, 65, 44, 65, 59, 61, 55, 44, 55), accentFill),
			shape(pl(35, 64, 52, 64, 52, 75, 39, 75, 35, 79), accentFill),
			line(pl(45, 12, 55, 12)),
			line(pl(44, 88, 56, 88)),
			line(quad(80, 20, 88, 28, 80, 36)),
			line(quad(86, 14, 98, 28, 86, 42)),
		}
	},
	"clapper": func() []dstroke {
		arm := rotate(pl(14, 30, 14, 20, 86, 20, 86, 30), -14, 14, 30)
		stripes := func(p []dpt) []dstroke { // slanted stripes across a bar
			a, b, c, d := p[0], p[1], p[2], p[3]
			var out []dstroke
			for _, t := range []float64{0.14, 0.34, 0.54, 0.74, 0.94} {
				u := t - 0.1
				out = append(out, line([]dpt{{a.x + (d.x-a.x)*u, a.y + (d.y-a.y)*u}, {b.x + (c.x-b.x)*t, b.y + (c.y-b.y)*t}}))
			}
			return out
		}
		s := []dstroke{
			shape(pl(14, 42, 86, 42, 86, 88, 14, 88), accentFill),
			shape(pl(14, 42, 14, 32, 86, 32, 86, 42), paperFill),
			shape(arm, paperFill),
		}
		s = append(s, stripes(pl(14, 42, 14, 32, 86, 32, 86, 42))...)
		s = append(s, stripes(arm)...)
		return append(s, line(pl(22, 56, 78, 56)), line(pl(22, 70, 50, 70)), line(pl(58, 70, 78, 70)))
	},
	"plate": func() []dstroke {
		return []dstroke{
			shape(star(50, 14, 11), accentFill),
			line(ring(50, 62, 27, 27)),
			line(ring(50, 62, 18, 18)),
			line(pl(12, 36, 12, 92)),
			line(pl(7, 36, 7, 50, 17, 50, 17, 36)),
			line(join(pl(88, 92, 88, 36), quad(88, 36, 96, 44, 88, 62))),
		}
	},
	"clock": func() []dstroke {
		s := []dstroke{
			shape(ring(50, 54, 33, 33), accentFill),
			line(pl(50, 54, 50, 33)),
			line(pl(50, 54, 65, 61)),
			dot(50, 54, 2.2),
			line(arc(28, 20, 10, 8, 150, 330)),
			line(arc(72, 20, 10, 8, 210, 390)),
			line(pl(30, 82, 22, 94)),
			line(pl(70, 82, 78, 94)),
		}
		for a := 0; a < 360; a += 90 {
			sn, cs := math.Sincos(float64(a) * math.Pi / 180)
			s = append(s, line(pl(50+26*cs, 54+26*sn, 50+30*cs, 54+30*sn)))
		}
		return s
	},
	"mic": func() []dstroke {
		return []dstroke{
			shape(join(arc(50, 24, 15, 15, 180, 360), pl(65, 44), arc(50, 44, 15, 15, 0, 180)), accentFill),
			line(pl(38, 26, 62, 26)),
			line(pl(38, 34, 62, 34)),
			line(pl(38, 42, 62, 42)),
			line(arc(50, 44, 24, 24, 180, 0)),
			line(pl(50, 68, 50, 88)),
			line(quad(32, 91, 50, 86, 68, 91)),
		}
	},
	"flame": func() []dstroke {
		outer := join(cubic(50, 94, 8, 90, 24, 50, 46, 6), cubic(46, 6, 50, 34, 86, 44, 76, 72), cubic(76, 72, 72, 88, 60, 94, 50, 94))
		inner := join(cubic(50, 90, 30, 88, 38, 66, 50, 48), cubic(50, 48, 52, 62, 68, 70, 62, 82), quad(62, 82, 58, 90, 50, 90))
		return []dstroke{shape(outer, accentFill), shape(inner, paperFill)}
	},
	"bulb": func() []dstroke {
		glass := join(arc(50, 40, 25, 25, 125, 415), pl(60, 72, 40, 72))
		s := []dstroke{
			shape(glass, accentFill),
			line(pl(41, 78, 59, 78)),
			line(pl(42, 84, 58, 84)),
			line(quad(45, 89, 50, 93, 55, 89)),
			line(pl(44, 70, 46, 52, 50, 58, 54, 52, 56, 70)),
		}
		for _, a := range []float64{180, 225, 270, 315, 360} {
			sn, cs := math.Sincos(a * math.Pi / 180)
			s = append(s, line(pl(50+33*cs, 40+33*sn, 50+41*cs, 40+41*sn)))
		}
		return s
	},
}

// DoodleNames lists every drawing, sorted.
func DoodleNames() []string {
	names := make([]string, 0, len(doodles))
	for n := range doodles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ---- drawing them

func wobble(s, seed float64) float64 {
	return 0.6*math.Sin(s*0.31+seed*12.9898) + 0.4*math.Sin(s*0.77+seed*78.233)
}

// rough resamples a path every grid unit and nudges each point a little, so
// straight lines waver like pen lines.
func rough(p []dpt, seed, amp float64) []dpt {
	var out []dpt
	var s float64
	for i := 0; i+1 < len(p); i++ {
		a, b := p[i], p[i+1]
		d := math.Hypot(b.x-a.x, b.y-a.y)
		n := int(math.Ceil(d))
		if n < 1 {
			n = 1
		}
		for k := 0; k < n; k++ {
			t := float64(k) / float64(n)
			at := s + d*t
			out = append(out, dpt{a.x + (b.x-a.x)*t + amp*wobble(at, seed), a.y + (b.y-a.y)*t + amp*wobble(at, seed+3.7)})
		}
		s += d
	}
	last := p[len(p)-1]
	return append(out, dpt{last.x + amp*wobble(s, seed), last.y + amp*wobble(s, seed+3.7)})
}

func pathLen(p []dpt) float64 {
	var l float64
	for i := 0; i+1 < len(p); i++ {
		l += math.Hypot(p[i+1].x-p[i].x, p[i+1].y-p[i].y)
	}
	return l
}

// Doodle draws the named drawing in a square of the given size with its top
// left corner at x, y. progress from 0 to 1 draws that much of it, stroke by
// stroke. boil picks the wobble: a different number gives the same drawing
// with slightly different lines. It reports false if there is no such drawing.
func Doodle(dc *gg.Context, name string, x, y, size float64, ink, accent, paper color.RGBA, progress float64, boil int) bool {
	make_, ok := doodles[name]
	if !ok {
		return false
	}
	if progress <= 0 {
		return true
	}
	strokes := make_()
	u := size / 100
	var total float64
	for _, s := range strokes {
		total += pathLen(s.pts)
	}
	budget := total * math.Min(progress, 1)

	dc.Push()
	defer dc.Pop()
	dc.SetLineCap(gg.LineCapRound)
	dc.SetLineJoin(gg.LineJoinRound)
	trace := func(p []dpt, dx, dy float64) {
		for i, q := range p {
			if i == 0 {
				dc.MoveTo(x+(q.x+dx)*u, y+(q.y+dy)*u)
			} else {
				dc.LineTo(x+(q.x+dx)*u, y+(q.y+dy)*u)
			}
		}
	}
	for i, s := range strokes {
		if budget <= 0 {
			break
		}
		length := pathLen(s.pts)
		done := math.Min(1, budget/math.Max(length, 0.001))
		budget -= length
		seed := float64(i)*1.618 + float64(boil)*0.73
		p := rough(s.pts, seed, 0.55)

		if s.fill != noFill {
			var c color.RGBA
			var off float64
			switch s.fill {
			case accentFill: // printed slightly out of register, like a cheap poster
				c, off = accent, 2.2
			case inkFill:
				c = ink
			case paperFill:
				c = paper
			}
			a := done * done
			if s.fill == paperFill {
				a = 1 // always hide what is behind, even while the outline is being drawn
			}
			dc.SetColor(color.NRGBA{c.R, c.G, c.B, uint8(255 * a)})
			trace(p, off, off)
			dc.ClosePath()
			dc.Fill()
		}

		part := p
		if done < 1 {
			part = p[:int(math.Max(2, math.Ceil(done*float64(len(p)))))]
		}
		line := ink
		if s.accent {
			line = accent
		}
		dc.SetColor(line)
		dc.SetLineWidth(math.Max(2.5, 1.9*u))
		trace(part, 0, 0)
		dc.Stroke()
		// a second, lighter pass a hair off the first, as when a pen goes over a line twice
		again := rough(s.pts, seed+9.1, 0.8)
		if done < 1 {
			again = again[:len(part)]
		}
		dc.SetColor(color.NRGBA{line.R, line.G, line.B, 110})
		dc.SetLineWidth(math.Max(1.5, 0.9*u))
		trace(again, 0, 0)
		dc.Stroke()
	}
	return true
}

// seriesDoodle is the drawing a series uses when a post names none.
var seriesDoodle = map[string]string{
	"ONE-LINER":      "mic",
	"HOT TAKE":       "flame",
	"RANDOM THOUGHT": "bulb",
}

// DoodleFor returns the drawing for a post, or "" if it has none.
func DoodleFor(p content.Post) string {
	if _, ok := doodles[p.Doodle]; ok {
		return p.Doodle
	}
	return seriesDoodle[p.Label]
}
