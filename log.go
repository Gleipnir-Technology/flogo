package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite

	colorBold     = 1
	colorDarkGray = 90

	unknownLevel = "???"
)

func setupLogging(file *os.File) zerolog.Logger {
	if os.Getenv("FLOGO_VERBOSE") != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Track start time for delta timestamps
	startTime := time.Now()

	writer := zerolog.ConsoleWriter{
		Out:        file,
		NoColor:    false,      // Enable colors for tail -f
		TimeFormat: "15:04:05", // placeholder, will be overridden
	}
	// Custom timestamp formatter showing elapsed time
	writer.FormatTimestamp = func(i any) string {
		elapsed := time.Since(startTime)

		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		seconds := int(elapsed.Seconds()) % 60
		millis := int(elapsed.Milliseconds()) % 1000

		return fmt.Sprintf("\x1b[90m[+%02d:%02d:%02d.%03d]\x1b[0m",
			hours, minutes, seconds, millis)
	}

	// Create logger with timestamp
	log.Logger = zerolog.New(writer).With().Timestamp().Caller().Logger()

	log.Debug().Msg("Running in verbose mode due to FLOGO_VERBOSE")
	return log.Logger
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true or c is 0.
func colorize(s interface{}, c int, disabled bool) string {
	e := os.Getenv("NO_COLOR")
	if e != "" || c == 0 {
		disabled = true
	}

	if disabled {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}
