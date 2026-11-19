// Command genicon draws Lastab's app and tray icons from the unsaved-tab
// mark (design/brand): a tab on its tab strip, marked with the dot an
// editor shows on an unsaved tab.
//
//	resources/icon.png                    1024×1024, ink tile, the app
//	resources/icon-dev.png                1024×1024, orange tile, the development app
//	internal/shell/assets/tray-template.png  36×36, black on clear (macOS menu bar, 18 pt @2x)
//	internal/shell/assets/tray-win.png       32×32, ink tile (Windows notification area)
//
// The geometry matches the SVG masters that design/brand/gen.mjs writes.
// Change both together.
//
//	go run ./scripts/genicon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

type rgba struct{ r, g, b, a float64 }

func hex(s string) rgba {
	var v [3]float64
	for i := range v {
		var n int
		for _, c := range s[1+2*i : 3+2*i] {
			n *= 16
			switch {
			case c >= '0' && c <= '9':
				n += int(c - '0')
			case c >= 'A' && c <= 'F':
				n += int(c-'A') + 10
			case c >= 'a' && c <= 'f':
				n += int(c-'a') + 10
			}
		}
		v[i] = float64(n) / 255
	}
	return rgba{v[0], v[1], v[2], 1}
}

// Brand colours, as in design/brand/README.md.
var (
	ink         = hex("#1C1C1A")
	white       = hex("#FFFFFF")
	black       = hex("#000000")
	blue        = hex("#2F5BD3")
	blueLift    = hex("#5B85F2") // the dot on ink
	greyOnInk   = hex("#6A6A63") // the tab title on ink
	greyOnWhite = hex("#C9C9C2") // the tab title on white
	line        = hex("#E3E3DD") // text lines on the page
	inactive    = hex("#34342F") // the inactive tab
	orange      = hex("#C2410C") // development builds
	orangeTab   = hex("#D2581F") // the inactive tab on orange
)

// A shape is a signed distance: negative inside, in the shape's own units.
type shape func(x, y float64) float64

func union(a, b shape) shape {
	return func(x, y float64) float64 { return math.Min(a(x, y), b(x, y)) }
}

// cut removes b from a.
func cut(a, b shape) shape {
	return func(x, y float64) float64 { return math.Max(a(x, y), -b(x, y)) }
}

func rect(x0, y0, x1, y1 float64) shape {
	return func(x, y float64) float64 { return roundRect(x, y, x0, y0, x1, y1, 0) }
}

// rrect is a rectangle with corner radius r.
func rrect(x0, y0, x1, y1, r float64) shape {
	return func(x, y float64) float64 { return roundRect(x, y, x0, y0, x1, y1, r) }
}

// pill is a rectangle with fully rounded ends.
func pill(x, y, w, h float64) shape { return rrect(x, y, x+w, y+h, h/2) }

func disc(cx, cy, r float64) shape {
	return func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - r }
}

// roundRect is the signed distance from (x, y) to a rounded rectangle.
func roundRect(x, y, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	dx, dy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	out := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	return out + math.Min(math.Max(dx, dy), 0) - r
}

// tab is a tab with top corners rt standing on a ground at y = yb, joined to
// it by concave fillets rf: the shape is filled from yt down to yb, and from
// x0-rf to x1+rf where it meets the ground. It reaches ov further down, into
// the ground drawn there, so the join leaves no antialiased seam.
func tab(x0, x1, yt, yb, rt, rf, ov float64) shape {
	body := rrect(x0, yt, x1, yb+rt, rt) // the bottom corners sink below yb
	clip := rect(-1e6, -1e6, 1e6, yb+ov)
	foot := cut(rect(x0-rf, yb-rf, x1+rf, yb+ov),
		union(disc(x0-rf, yb-rf, rf), disc(x1+rf, yb-rf, rf)))
	return func(x, y float64) float64 {
		return math.Min(math.Max(body(x, y), clip(x, y)), foot(x, y))
	}
}

// The symbol on its 256 grid; the content box is 32..224 by 64..192.
var (
	symTab   = union(tab(64, 192, 64, 168, 36, 20, 12), pill(32, 168, 192, 24))
	symTitle = pill(82, 107, 56, 18)
	symDot   = disc(166, 116, 14)
	// Below 24 px the title is dropped and the dot grows.
	symDotSmall = disc(128, 116, 24)
)

type layer struct {
	dist  shape // in pixels
	color rgba
}

// scaled places a shape drawn on a grid of g units into a size-pixel image.
func scaled(f shape, g, size float64) shape {
	s := size / g
	return func(x, y float64) float64 { return f(x/s, y/s) * s }
}

// at places a shape from the 256 symbol grid, whose content box is 192
// wide, centred on (cx, cy) and w pixels wide.
func at(f shape, cx, cy, w float64) shape {
	s := w / 192
	return func(x, y float64) float64 { return f(128+(x-cx)/s, 128+(y-cy)/s) * s }
}

// appIcon is the app icon on the macOS 1024 grid: the tile's top is the
// tab strip, one tab is open and runs into the page, whose last line holds
// the cursor; an inactive tab sits behind on the right.
func appIcon(size float64, strip, back rgba) []layer {
	tile := rrect(100, 100, 924, 924, 185)
	in := func(f shape) shape {
		return scaled(func(x, y float64) float64 { return math.Max(f(x, y), tile(x, y)) }, 1024, size)
	}
	page := union(tab(212, 632, 252, 476, 96, 56, 40), rect(0, 476, 1024, 1024))
	inactiveTab := union(rrect(716, 316, 1024+120, 476+120, 60), rect(1024, 316, 1100, 476))
	return []layer{
		{in(rect(0, 0, 1024, 1024)), strip},
		{in(inactiveTab), back},
		{in(page), white},
		{in(pill(292, 342, 170, 52)), greyOnWhite},
		{in(disc(548, 368, 50)), blue},
		{in(pill(212, 624, 300, 52)), line},
		{in(pill(212, 728, 190, 52)), line},
		{in(rrect(430, 700, 466, 808, 18)), blue},
	}
}

func tile(x0, y0, x1, y1, r float64, c rgba) layer {
	return layer{func(x, y float64) float64 { return roundRect(x, y, x0, y0, x1, y1, r) }, c}
}

func over(dst, src rgba, a float64) rgba {
	a *= src.a
	out := dst.a + a*(1-dst.a)
	if out == 0 {
		return rgba{}
	}
	f := func(d, s float64) float64 { return (s*a + d*dst.a*(1-a)) / out }
	return rgba{f(dst.r, src.r), f(dst.g, src.g), f(dst.b, src.b), out}
}

func draw(path string, size int, layers []layer) error {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4 // samples per side of a pixel
	for py := range size {
		for px := range size {
			var acc rgba
			for sy := range ss {
				for sx := range ss {
					x := float64(px) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					var c rgba
					for _, l := range layers {
						if d := l.dist(x, y); d < 0.5 {
							c = over(c, l.color, math.Min(0.5-d, 1))
						}
					}
					acc.r += c.r * c.a
					acc.g += c.g * c.a
					acc.b += c.b * c.a
					acc.a += c.a
				}
			}
			if acc.a == 0 {
				continue
			}
			n := float64(ss * ss)
			to8 := func(v float64) uint8 { return uint8(math.Round(math.Min(v, 1) * 255)) }
			img.SetNRGBA(px, py, color.NRGBA{to8(acc.r / acc.a), to8(acc.g / acc.a), to8(acc.b / acc.a), to8(acc.a / n)})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func main() {
	// The menu bar template is the symbol in black with the title and dot
	// cut out; at 18 pt the small form keeps the dot legible.
	template := cut(cut(symTab, symTitle), symDot)
	icons := []struct {
		path   string
		size   int
		layers []layer
	}{
		{"resources/icon.png", 1024, appIcon(1024, ink, inactive)},
		{"resources/icon-dev.png", 1024, appIcon(1024, orange, orangeTab)},
		// Template images are drawn black; the menu bar tints them.
		{"internal/shell/assets/tray-template.png", 36, []layer{{at(template, 18, 18, 34), black}}},
		{"internal/shell/assets/tray-win.png", 32, []layer{
			tile(0, 0, 32, 32, 7, ink),
			{at(symTab, 16, 16, 26), white},
			{at(symDotSmall, 16, 16, 26), blue},
		}},
	}
	for _, ic := range icons {
		if err := os.MkdirAll(filepath.Dir(ic.path), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := draw(ic.path, ic.size, ic.layers); err != nil {
			log.Fatal(err)
		}
	}
}
