package main

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/leaanthony/go-ansi-parser"
	//"github.com/rs/zerolog/log"
)

// DrawStyledText renders styled segments to the tcell screen
func DrawStyledText(s tcell.Screen, x, y int, text []*ansi.StyledText) {
	col := x
	row := y
	for _, seg := range text {
		style := convertStyle(seg)
		for _, r := range seg.Label {
			s.SetContent(col, row, r, nil, style)
			col++
			if r == '\n' {

				col = x
				row = row + 1
			}
		}
	}
}

func ParseANSI(buf []byte) ([]*ansi.StyledText, error) {
	//log.Debug().Bytes("buf", buf).Send()
	return ansi.Parse(string(buf))
}

func StripColorCodes(buf []byte) string {
	result, err := ansi.Cleanse(string(buf), ansi.WithIgnoreInvalidCodes())
	if err != nil {
		return fmt.Sprintf("Failed to strip color codes: %v", err)
	}
	return result
}
func convertStyle(t *ansi.StyledText) tcell.Style {
	style := tcell.StyleDefault
	if t == nil {
		return style
	}
	if t.FgCol != nil {
		style = style.Foreground(color.NewRGBColor(
			int32(t.FgCol.Rgb.R),
			int32(t.FgCol.Rgb.G),
			int32(t.FgCol.Rgb.B),
		))
	}
	if t.BgCol != nil {
		style = style.Background(color.NewRGBColor(
			int32(t.BgCol.Rgb.R),
			int32(t.BgCol.Rgb.G),
			int32(t.BgCol.Rgb.B),
		))
	}
	if t.Blinking() {
		style = style.Blink(true)
	}
	if t.Bold() {
		style = style.Bold(true)
	}
	if t.Faint() {
		style = style.Dim(true)
	}
	//if t.Inversed() {
	//style = style.Invert(true)
	//}
	if t.Italic() {
		style = style.Italic(true)
	}
	if t.Strikethrough() {
		style = style.StrikeThrough(true)
	}
	if t.Underlined() {
		style = style.Underline(true)
	}
	return style
}
