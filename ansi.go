package main

import (
	"github.com/gdamore/tcell/v3"
	"github.com/leaanthony/go-ansi-parser"

	"github.com/rs/zerolog/log"
)

// DrawStyledText renders styled segments to the tcell screen
func DrawStyledText(s tcell.Screen, x, y int, text []*ansi.StyledText) {
	col := x
	row := y
	for _, seg := range text {
		log.Debug().Str("text", seg.Label).Send()
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
	log.Debug().Bytes("buf", buf).Send()
	return ansi.Parse(string(buf))
}

func convertStyle(t *ansi.StyledText) tcell.Style {
	if t == nil || t.FgCol == nil {
		return tcell.StyleDefault
	}
	result := tcell.StyleDefault.Foreground(
		tcell.NewRGBColor(
			int32(t.FgCol.Rgb.R),
			int32(t.FgCol.Rgb.G),
			int32(t.FgCol.Rgb.B),
		),
	)
	if t.BgCol == nil {
		return result
	}
	return result.Background(
		tcell.NewRGBColor(
			int32(t.BgCol.Rgb.R),
			int32(t.BgCol.Rgb.G),
			int32(t.BgCol.Rgb.B),
		),
	)
}
