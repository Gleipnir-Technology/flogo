package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/leaanthony/go-ansi-parser"
	"github.com/rs/zerolog/log"
)

func DrawBytesMultiline(s tcell.Screen, start_x, start_y int, style tcell.Style, buffer []byte) {
	parsed, err := ansi.Parse(string(buffer))

	// Convert the buffer into ansi sequences with newlines and wrapping
	max_x, max_y := s.Size()
	sections, err := fitToScreen(start_x, start_y, max_x, max_y, parsed)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse ANSI")
		return
	}
	drawStyledText(s, start_x, start_y, sections)
}

// drawStyledText renders styled segments to the tcell screen
func drawStyledText(s tcell.Screen, x_start, y_start int, lines [][]*ansi.StyledText) {
	x := x_start
	_, y_max := s.Size()
	// We draw the lines in reverse order so we ensure we are seeing the latest output
	y_count := y_max - y_start
	lines_len := len(lines)
	for y_offset := range y_count {
		idx_base := max(lines_len-y_count, 0)
		idx := idx_base + y_offset
		if idx >= lines_len {
			return
		}
		//log.Debug().Int("idx", idx).Int("lines_len", lines_len).Int("y_offset", y_offset).Send()
		line := lines[idx]
		y := y_start + y_offset
		for _, seg := range line {
			style := convertStyle(seg)
			/*log.Debug().
			Str("fg", style.GetForeground().String()).
			Str("bg", style.GetBackground().String()).
			Send()*/
			for _, r := range seg.Label {
				s.SetContent(x, y, r, nil, style)
				x++
			}
		}
		x = x_start
	}
}

func fitToScreen(start_x, start_y, max_x, max_y int, parsed []*ansi.StyledText) ([][]*ansi.StyledText, error) {
	lines := make([][]*ansi.StyledText, 0)
	current_line := make([]*ansi.StyledText, 0)
	cur_label := strings.Builder{}
	i := start_x
	for _, section := range parsed {
		debugLogANSI(section)
		for _, r := range section.Label {
			if r == '\n' || i > max_x {
				current_line = append(current_line, &ansi.StyledText{
					Label:      cur_label.String(),
					FgCol:      section.FgCol,
					BgCol:      section.BgCol,
					Style:      section.Style,
					ColourMode: section.ColourMode,
					Offset:     section.Offset,
					Len:        len(cur_label.String()),
				})
				i = start_x
				lines = append(lines, current_line)
				current_line = make([]*ansi.StyledText, 0)
				cur_label.Reset()
			} else {
				cur_label.WriteRune(r)
			}
		}
		current_line = append(current_line, &ansi.StyledText{
			Label:      cur_label.String(),
			FgCol:      section.FgCol,
			BgCol:      section.BgCol,
			Style:      section.Style,
			ColourMode: section.ColourMode,
			Offset:     section.Offset,
			Len:        len(cur_label.String()),
		})
		cur_label.Reset()
	}
	lines = append(lines, current_line)
	log.Debug().Int("len.lines", len(lines)).Int("len.parsed", len(parsed)).Msg("fit to screen")
	return lines, nil
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
