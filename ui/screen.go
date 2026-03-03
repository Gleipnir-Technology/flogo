package ui

import (
	"strings"

	"github.com/rs/zerolog/log"
)

func fitToScreen(start_x, start_y, max_x, max_y int, text []*styledText) ([][]*styledText, error) {
	lines := make([][]*styledText, 0)
	current_line := make([]*styledText, 0)
	cur_text := strings.Builder{}
	x := start_x
	log.Debug().Int("start_x", start_x).Int("start_y", start_x).Int("max_x", max_x).Int("max_y", max_y).Int("len", len(text)).Msg("fitting to screen")
	for i, section := range text {
		for j, r := range section.text {
			if r == '\n' || x > max_x {
				if r == '\n' {
					log.Debug().Int("i", i).Int("j", j).Int("x", x).Msg("newline break")
				} else if x > max_x {
					log.Debug().Int("i", i).Int("j", j).Int("x", x).Msg("overflow x")
				}
				current_line = append(current_line, &styledText{
					style: section.style,
					text:  cur_text.String(),
				})
				x = start_x
				lines = append(lines, current_line)
				current_line = make([]*styledText, 0)
				cur_text.Reset()
			} else {
				cur_text.WriteRune(r)
				x++
			}
		}
		current_line = append(current_line, &styledText{
			style: section.style,
			text:  cur_text.String(),
		})
		cur_text.Reset()
	}
	lines = append(lines, current_line)
	log.Debug().Int("len.lines", len(lines)).Msg("done fit to screen")
	return lines, nil
}
