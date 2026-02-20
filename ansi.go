package main

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/leaanthony/go-ansi-parser"
)

func applyANSICodes(style tcell.Style, codes string) tcell.Style {
	if codes == "" || codes == "0" {
		return tcell.StyleDefault
	}

	parts := strings.Split(codes, ";")

	for _, part := range parts {
		code, _ := strconv.Atoi(part)

		switch code {
		case 0: // Reset
			style = tcell.StyleDefault
		case 1: // Bold
			style = style.Bold(true)
		case 4: // Underline
			style = style.Underline(true)
		case 7: // Reverse
			style = style.Reverse(true)

		// Foreground colors (30-37)
		case 30:
			style = style.Foreground(tcell.ColorBlack)
		case 31:
			style = style.Foreground(tcell.ColorMaroon)
		case 32:
			style = style.Foreground(tcell.ColorGreen)
		case 33:
			style = style.Foreground(tcell.ColorOlive)
		case 34:
			style = style.Foreground(tcell.ColorNavy)
		case 35:
			style = style.Foreground(tcell.ColorPurple)
		case 36:
			style = style.Foreground(tcell.ColorTeal)
		case 37:
			style = style.Foreground(tcell.ColorSilver)

		// Background colors (40-47)
		case 40:
			style = style.Background(tcell.ColorBlack)
		case 41:
			style = style.Background(tcell.ColorMaroon)
		case 42:
			style = style.Background(tcell.ColorGreen)
		case 43:
			style = style.Background(tcell.ColorOlive)
		case 44:
			style = style.Background(tcell.ColorNavy)
		case 45:
			style = style.Background(tcell.ColorPurple)
		case 46:
			style = style.Background(tcell.ColorTeal)
		case 47:
			style = style.Background(tcell.ColorSilver)
		// Bright foreground colors (90-97)
		case 90:
			style = style.Foreground(tcell.ColorGray)
		case 91:
			style = style.Foreground(tcell.ColorRed)
		case 92:
			style = style.Foreground(tcell.ColorLime)
		case 93:
			style = style.Foreground(tcell.ColorYellow)
		case 94:
			style = style.Foreground(tcell.ColorBlue)
		case 95:
			style = style.Foreground(tcell.ColorFuchsia)
		case 96:
			style = style.Foreground(tcell.ColorAqua)
		case 97:
			style = style.Foreground(tcell.ColorWhite)
		}

	}

	return style
}

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
			}
		}
		col = x
		row = row + 1
	}
}

func ParseANSI(buf []byte) ([]*ansi.StyledText, error) {
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
