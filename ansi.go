package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/leaanthony/go-ansi-parser"
	"github.com/rs/zerolog/log"
)

// DrawStyledText renders styled segments to the tcell screen
func DrawStyledText(s tcell.Screen, start_x, start_y int, text []*ansi.StyledText) {
	col := start_x
	row := start_y
	max_x, _ := s.Size()
	for _, seg := range text {
		style := convertStyle(seg)
		for _, r := range seg.Label {
			s.SetContent(col, row, r, nil, style)
			col++
			if r == '\n' {
				col = start_x
				row = row + 1
			} else if col > max_x {
				col = start_x
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
		style = style.Foreground(convertStyleColor(style, t.FgCol))
	}
	if t.BgCol != nil {
		style = style.Background(convertStyleColor(style, t.BgCol))
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
func convertStyleColor(style tcell.Style, c *ansi.Col) color.Color {
	result := color.GetColor(strings.ToLower(c.Name))
	if result.Valid() {
		return result
	}
	log.Debug().
		Int("id", c.Id).
		Str("hex", c.Hex).
		Uint("r", uint(c.Rgb.R)).
		Uint("g", uint(c.Rgb.G)).
		Uint("b", uint(c.Rgb.B)).
		Str("name", c.Name).
		Msg("color fallback")
	return color.NewRGBColor(
		int32(c.Rgb.R),
		int32(c.Rgb.G),
		int32(c.Rgb.B),
	)
}
