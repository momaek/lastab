// Package theme builds the window's ui.Theme from the design tokens: the
// palette of the appearance, the accent and its derived shades, radii and
// spacing. Views read colors from the Palette, never literals.
package theme

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Font families, from tokens.json: Inter and JetBrains Mono (embedded by
// resources/fonts), Chinese falling back to the system's.
const (
	UIFamily   = "Inter, PingFang SC, Microsoft YaHei"
	MonoFamily = "JetBrains Mono, PingFang SC, Microsoft YaHei"
	// MonoEditor is the editor's default: JetBrains Mono alone, Chinese
	// falling back as the editor picks.
	MonoEditor = "JetBrains Mono"
)

// NoLigatures keeps code as typed: JetBrains Mono draws ->, != and ==
// as one glyph unless contextual alternates and ligatures are off.
const NoLigatures = "calt=0,liga=0"

// Derived are the accent's shades from tokens.json's rules.
type Derived struct {
	Accent   ui.Color
	OnAccent ui.Color // white or #111111, whichever contrasts more
	Sel      ui.Color // selected rows, active nav item
	SelLine  ui.Color // selected card outline
}

// For returns the palette of an appearance.
func For(dark bool) *Palette {
	if dark {
		return &Dark
	}
	return &Light
}

// Accent returns the accent for an appearance: presets are lifted on dark.
func Accent(hex string, dark bool) ui.Color {
	if hex == "" {
		hex = DefaultAccent
	}
	if dark {
		if l, ok := DarkLift[strings.ToUpper(hex)]; ok {
			hex = l
		}
	}
	return ui.Hex(hex)
}

// Derive computes the accent's shades over a palette.
func Derive(p *Palette, accent ui.Color, dark bool) Derived {
	sel, line := float32(0.10), float32(0.30)
	if dark {
		sel, line = 0.20, 0.45
	}
	on := ui.Hex("#FFFFFF")
	if contrast(accent, ui.Hex("#111111")) > contrast(accent, on) {
		on = ui.Hex("#111111")
	}
	return Derived{
		Accent:   accent,
		OnAccent: on,
		Sel:      p.Bg.Mix(accent, sel),
		SelLine:  p.Bg.Mix(accent, line),
	}
}

// Build returns the ui.Theme of the window.
func Build(dark bool, accentHex string) (*ui.Theme, *Palette, Derived) {
	p := For(dark)
	base := ui.LightTheme()
	if dark {
		base = ui.DarkTheme()
	}
	acc := Accent(accentHex, dark)
	d := Derive(p, acc, dark)
	t := *base.WithAccent(acc)
	t.Background = p.Bg
	t.Surface = p.Btn
	t.Border = p.BtnLine
	t.Text = p.Text
	t.TextMuted = p.Muted
	t.Danger = p.Danger
	t.AccentText = d.OnAccent
	t.Selection = p.Bg.Mix(acc, map[bool]float32{false: 0.22, true: 0.38}[dark])
	t.Inverse = p.Toast
	t.InverseText = p.ToastText
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = UIFontSize
	t.Font = UIFamily
	return &t, p, d
}

func luminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
