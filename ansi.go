package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/rs/zerolog/log"
)

// ParseANSI parses a buffer with ANSI escape sequences and returns
// segments with their associated styles
type StyledSegment struct {
	Text  string
	Style tcell.Style
}

func ParseANSI(input string) []StyledSegment {
	var segments []StyledSegment
	currentStyle := tcell.StyleDefault

	// Regex to match ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[([0-9;]*)m`)

	lastEnd := 0
	matches := ansiRegex.FindAllStringSubmatchIndex(input, -1)

	for _, match := range matches {
		start, end := match[0], match[1]
		codeStart, codeEnd := match[2], match[3]

		// Add text before this escape sequence
		if start > lastEnd {
			text := input[lastEnd:start]
			if text != "" {
				segments = append(segments, StyledSegment{
					Text:  text,
					Style: currentStyle,
				})
			}
		}

		// Parse the escape code and update style
		if codeStart != -1 {
			codes := input[codeStart:codeEnd]
			currentStyle = applyANSICodes(currentStyle, codes)
		}

		lastEnd = end
	}

	// Add remaining text
	if lastEnd < len(input) {
		text := input[lastEnd:]
		if text != "" {
			segments = append(segments, StyledSegment{
				Text:  text,
				Style: currentStyle,
			})
		}
	}

	return segments
}

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
		}
	}

	return style
}

// DrawStyledText renders styled segments to the tcell screen
func DrawStyledText(s tcell.Screen, x, y int, segments []StyledSegment) {
	col := x
	row := y
	for _, seg := range segments {
		for _, r := range seg.Text {
			s.SetContent(col, row, r, nil, seg.Style)
			col++
			if r == '\n' {
			}
		}
		col = x
		row = row + 1
	}
}
