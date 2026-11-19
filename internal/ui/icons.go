package appui

import "github.com/egoist/mygo/ui"

// The app's icons: outlines on a 16×16 grid, 1.5 wide with round ends,
// drawn in the color of the text around them. They replace text symbols
// such as ⚙ or ⌕, which each font draws in its own size and weight and
// which some fonts lack.
var (
	iconPlus      = strokedIcon(`<path d="M8 3v10M3 8h10"/>`)
	iconMinus     = strokedIcon(`<path d="M3 8h10"/>`)
	iconClose     = strokedIcon(`<path d="M5 5l6 6M11 5l-6 6"/>`)
	iconSearch    = strokedIcon(`<path d="M7 11.5a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9zM10.5 10.5 14 14"/>`)
	iconRecent    = strokedIcon(`<path d="M8 14A6 6 0 1 0 8 2a6 6 0 0 0 0 12zM8 5v3l2 1.5"/>`)
	iconHistory   = strokedIcon(`<path d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9M2.5 2.5V5H5M8 5.5V8l1.8 1.2"/>`)
	iconSettings  = strokedIcon(`<path d="M6.81 3.25 7.11 1.66 8.89 1.66 9.19 3.25 10.52 3.8 11.85 2.89 13.11 4.15 12.2 5.48 12.75 6.81 14.34 7.11 14.34 8.89 12.75 9.19 12.2 10.52 13.11 11.85 11.85 13.11 10.52 12.2 9.19 12.75 8.89 14.34 7.11 14.34 6.81 12.75 5.48 12.2 4.15 13.11 2.89 11.85 3.8 10.52 3.25 9.19 1.66 8.89 1.66 7.11 3.25 6.81 3.8 5.48 2.89 4.15 4.15 2.89 5.48 3.8z"/><circle cx="8" cy="8" r="2"/>`)
	iconHideSide  = strokedIcon(`<rect x="2" y="2.5" width="12" height="11" rx="2"/><path d="M6 2.5v11M11 6.25 9.25 8 11 9.75"/>`)
	iconShowSide  = strokedIcon(`<rect x="2" y="2.5" width="12" height="11" rx="2"/><path d="M6 2.5v11M9.25 6.25 11 8 9.25 9.75"/>`)
	iconRename    = strokedIcon(`<path d="M10.5 3 13 5.5 6 12.5 3 13l.5-3z"/>`)
	iconText      = strokedIcon(`<path d="M3 4h10M3 8h10M3 12h6"/>`)
	iconInfo      = strokedIcon(`<circle cx="8" cy="8" r="6"/><path d="M8 7.5V11M8 5h.01"/>`)
	iconCheck     = strokedIcon(`<path d="M3.5 8.5l3 3 6-7"/>`)
	iconChevDown  = strokedIcon(`<path d="M4 6l4 4 4-4"/>`)
	iconChevRight = strokedIcon(`<path d="M6 4l4 4-4 4"/>`)
	iconTodo      = strokedIcon(`<rect x="2.5" y="2.5" width="11" height="11" rx="2.5"/><path d="M5.5 8l1.8 1.8L10.5 6"/>`)
)

// iconGrip is the handle that drags a to-do item: two columns of dots.
var iconGrip = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="14" viewBox="0 0 10 14" fill="currentColor"><circle cx="3" cy="3" r="1.2"/><circle cx="7" cy="3" r="1.2"/><circle cx="3" cy="7" r="1.2"/><circle cx="7" cy="7" r="1.2"/><circle cx="3" cy="11" r="1.2"/><circle cx="7" cy="11" r="1.2"/></svg>`))

func strokedIcon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}
