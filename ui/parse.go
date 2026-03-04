package ui

import (
	"regexp"
	"strconv"
	"strings"
)

type BuildOutputLineGo struct {
	Filename string
	Line     int
	Column   int
	Message  string
}

// ParseGoBuildOutput parses the output from go build into structured data
func parseGoBuildOutput(output string) ([]BuildOutputLineGo, error) {
	// Pattern: filename:line:column: message
	pattern := regexp.MustCompile(`^\s*(.+?):(\d+):(\d+):\s*(.+)$`)

	lines := strings.Split(output, "\n")
	result := make([]BuildOutputLineGo, 0)

	for _, line := range lines {
		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			result = append(result, BuildOutputLineGo{
				Filename: "none",
				Line:     0,
				Column:   0,
				Message:  line,
			})
			continue
		}

		// Parse line number
		lineNum, err := strconv.Atoi(matches[2])
		if err != nil {
			continue
		}

		// Parse column number
		colNum, err := strconv.Atoi(matches[3])
		if err != nil {
			continue
		}

		result = append(result, BuildOutputLineGo{
			Filename: matches[1],
			Line:     lineNum,
			Column:   colNum,
			Message:  matches[4],
		})
	}

	return result, nil
}
