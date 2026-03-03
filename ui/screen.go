package ui

import (
	"strings"

	"github.com/rs/zerolog/log"
)

func fitToScreen(start_x, start_y, max_x, max_y int, text []*styledText) ([][]*styledText, error) {
	lines := make([][]*styledText, 0)
	current_line := make([]*styledText, 0)
	cur_text := strings.Builder{}
	i := start_x
	for _, section := range text {
		for _, r := range section.text {
			if r == '\n' || i > max_x {
				current_line = append(current_line, &styledText{
					style: section.style,
					text:  cur_text.String(),
				})
				i = start_x
				lines = append(lines, current_line)
				current_line = make([]*styledText, 0)
				cur_text.Reset()
			} else {
				cur_text.WriteRune(r)
			}
		}
		current_line = append(current_line, &styledText{
			style: section.style,
			text:  cur_text.String(),
		})
		cur_text.Reset()
	}
	lines = append(lines, current_line)
	log.Debug().Int("len.lines", len(lines)).Int("len.parsed", len(lines)).Msg("fit to screen")
	return lines, nil
}
