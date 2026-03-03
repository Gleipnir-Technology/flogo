package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/leaanthony/go-ansi-parser"
	"github.com/rs/zerolog/log"
)

func ansiToTcell(st []*ansi.StyledText) ([]*styledText, error) {
	result := make([]*styledText, len(st))
	for i, section := range st {
		result[i] = &styledText{
			style: convertStyle(section),
			text:  section.Label,
		}
	}
	return result, nil
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
		log.Debug().Msg("converting nil style")
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
	log.Debug().
		Int("id", c.Id).
		Str("hex", c.Hex).
		Uint("r", uint(c.Rgb.R)).
		Uint("g", uint(c.Rgb.G)).
		Uint("b", uint(c.Rgb.B)).
		Str("name", c.Name).
		Msg("color fallback")
	result := color.GetColor(strings.ToLower(c.Name))
	if result.Valid() {
		return result
	}
	return color.NewRGBColor(
		int32(c.Rgb.R),
		int32(c.Rgb.G),
		int32(c.Rgb.B),
	)
}
func debugLogANSI(s *ansi.StyledText) {
	fg := "nil"
	bg := "nil"
	if s.FgCol != nil {
		fg = s.FgCol.Hex
	}
	if s.BgCol != nil {
		bg = s.BgCol.Hex
	}
	log.Debug().Str("fg", fg).Str("bg", bg).Str("label", s.Label).Send()
}
