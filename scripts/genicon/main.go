// Command genicon draws Lastab's placeholder app icons, resources/icon.png
// and resources/icon-dev.png (1024×1024): a sheet of paper with a tab and a
// text cursor on a rounded square, the development one in orange.
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

const size = 1024

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

func mix(a, b rgba, t float64) rgba {
	return rgba{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t, 1}
}

// roundRect is the signed distance from (x, y) to a rounded rectangle.
func roundRect(x, y, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	dx, dy := math.Abs(x-cx)-hx, math.Abs(y-cy)-hy
	out := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	return out + math.Min(math.Max(dx, dy), 0) - r
}

type layer struct {
	dist  func(x, y float64) float64
	color func(x, y float64) rgba
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

func draw(path string, top, bottom rgba) error {
	solid := func(c rgba) func(x, y float64) rgba { return func(float64, float64) rgba { return c } }
	paper := hex("#FFFFFF")
	line := hex("#C9D3E6")
	layers := []layer{
		// The rounded square of the macOS icon grid, 824 of 1024.
		{func(x, y float64) float64 { return roundRect(x, y, 100, 100, 924, 924, 185) },
			func(x, y float64) rgba { return mix(top, bottom, (y-100)/824) }},
		// Shadow of the sheet.
		{func(x, y float64) float64 { return roundRect(x, y, 262, 266, 762, 812, 44) - 6 },
			solid(rgba{0, 0, 0, 0.10})},
		// The tab on top of the sheet, then the sheet.
		{func(x, y float64) float64 { return roundRect(x, y, 262, 210, 520, 330, 36) }, solid(paper)},
		{func(x, y float64) float64 { return roundRect(x, y, 262, 250, 762, 790, 44) }, solid(paper)},
		// Lines of text, and the cursor.
		{func(x, y float64) float64 { return roundRect(x, y, 332, 360, 692, 392, 16) }, solid(line)},
		{func(x, y float64) float64 { return roundRect(x, y, 332, 452, 642, 484, 16) }, solid(line)},
		{func(x, y float64) float64 { return roundRect(x, y, 332, 544, 562, 576, 16) }, solid(line)},
		{func(x, y float64) float64 { return roundRect(x, y, 584, 520, 610, 600, 13) }, solid(bottom)},
	}
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
							c = over(c, l.color(x, y), math.Min(0.5-d, 1))
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
	dir := "resources"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := draw(filepath.Join(dir, "icon.png"), hex("#4A74E8"), hex("#2F5BD3")); err != nil {
		log.Fatal(err)
	}
	if err := draw(filepath.Join(dir, "icon-dev.png"), hex("#F08A3C"), hex("#C2410C")); err != nil {
		log.Fatal(err)
	}
}
