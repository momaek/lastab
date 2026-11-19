// Command genicon draws Lastab's app and tray icons from the back-tab mark
// (design/brand): a text cursor, and an arrow returning to it.
//
//	resources/icon.png                    1024×1024, ink tile, the app
//	resources/icon-dev.png                1024×1024, orange tile, the development app
//	internal/shell/assets/tray-template.png  36×36, black on clear (macOS menu bar, 18 pt @2x)
//	internal/shell/assets/tray-win.png       32×32, ink tile (Windows notification area)
//
// The geometry matches the SVG masters in design/brand: on a 256 grid, every
// stroke is a 32-wide capsule. Change both together.
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
	ink      = hex("#1C1C1A")
	white    = hex("#FFFFFF")
	black    = hex("#000000")
	blueLift = hex("#5B85F2") // the cursor on dark
	orange   = hex("#C2410C") // development builds
)

// roundRect is the signed distance from (x, y) to a rounded rectangle.
func roundRect(x, y, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	dx, dy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	out := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	return out + math.Min(math.Max(dx, dy), 0) - r
}

// capsule is the signed distance from (x, y) to a segment with round ends.
func capsule(x, y, ax, ay, bx, by, r float64) float64 {
	px, py, vx, vy := x-ax, y-ay, bx-ax, by-ay
	t := math.Max(0, math.Min(1, (px*vx+py*vy)/(vx*vx+vy*vy)))
	return math.Hypot(px-vx*t, py-vy*t) - r
}

// The mark on its 256 grid; the content box is 40..216 both ways.
const (
	grid    = 128.0 // centre of the grid
	content = 176.0 // height of the content box
	stroke  = 16.0  // half the stroke width
)

func cursor(x, y float64) float64 { return capsule(x, y, 56, 56, 56, 200, stroke) }

func arrow(x, y float64) float64 {
	return math.Min(capsule(x, y, 104, 128, 192, 128, stroke),
		math.Min(capsule(x, y, 152, 80, 104, 128, stroke), capsule(x, y, 104, 128, 152, 176, stroke)))
}

type layer struct {
	dist  func(x, y float64) float64
	color rgba
}

// mark places the mark centred on (cx, cy), its content box h pixels tall.
func mark(cx, cy, h float64, cursorColor, arrowColor rgba) []layer {
	s := h / content
	at := func(f func(x, y float64) float64) func(x, y float64) float64 {
		return func(x, y float64) float64 { return f(grid+(x-cx)/s, grid+(y-cy)/s) * s }
	}
	return []layer{{at(cursor), cursorColor}, {at(arrow), arrowColor}}
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
	// The macOS icon grid: a rounded square of 824 in 1024; the mark is 470 tall.
	app := func(bg, cursorColor rgba) []layer {
		return append([]layer{tile(100, 100, 924, 924, 185, bg)}, mark(512, 512, 470, cursorColor, white)...)
	}
	icons := []struct {
		path   string
		size   int
		layers []layer
	}{
		{"resources/icon.png", 1024, app(ink, blueLift)},
		{"resources/icon-dev.png", 1024, app(orange, ink)},
		// Template images are drawn black; the menu bar tints them.
		{"internal/shell/assets/tray-template.png", 36, mark(18, 18, 30, black, black)},
		{"internal/shell/assets/tray-win.png", 32,
			append([]layer{tile(0, 0, 32, 32, 7, ink)}, mark(16, 16, 20, blueLift, white)...)},
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
