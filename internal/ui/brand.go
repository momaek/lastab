package appui

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// The Lastab marks, which design/brand/gen.mjs writes: the tab and the
// wordmark take the color of the text around them, the dot stays blue and
// follows the appearance.
var (
	//go:embed brand/symbol.svg
	symbolSVG []byte
	//go:embed brand/horizontal.svg
	horizontalSVG []byte

	brandSymbol     = ui.MustParseSVG(symbolSVG)
	brandHorizontal = ui.MustParseSVG(horizontalSVG)
)

// brandMark shows the horizontal mark h DIPs tall, in color col.
func brandMark(c *ui.Context, h float32, col ui.Color) ui.Element {
	w, sh := ui.ImageSize(brandHorizontal)
	return ui.Image(c, brandHorizontal).Size(h*w/sh, h).TextColor(col).Label("Lastab")
}

// brandIcon shows the symbol h DIPs tall, in color col.
func brandIcon(c *ui.Context, h float32, col ui.Color) ui.Element {
	return ui.Image(c, brandSymbol).Size(h, h).TextColor(col).Label("Lastab")
}
