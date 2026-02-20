package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	var err error
	var target = flag.String("target", ".", "The directory containing the go project to build")
	flag.Parse()

	file, err := os.OpenFile(
		"flogo.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		fmt.Println("Failed to open 'flogo.log' for writing.")
		os.Exit(1)
	}
	defer file.Close()
	setupLogging(file)

	bind := os.Getenv("FLOGO_BIND")
	if bind == "" {
		bind = ":10000" // Default if not specified
	}
	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:9001" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		fmt.Printf("Failed to parse '%s' as a URL: %v\n", upstream, err)
		os.Exit(2)
	}
	// Handle keyboard input
	//c := make(chan os.Signal, 1)
	//signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	log.Info().Msg("entering main loop")

	// Keep track of the state of everything
	mgr := newFlogoStateManager()

	err = mgr.Run(bind, *target)
	if err != nil {
		fmt.Println("%+v", err)
		os.Exit(1)
	}
}
func reload() {
	log.Info().Msg("fake reload")
}
func setupLogging(file *os.File) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// Track start time for delta timestamps
	startTime := time.Now()

	writer := zerolog.ConsoleWriter{
		Out:        file,
		TimeFormat: "15:04:05", // placeholder, will be overridden
		NoColor:    false,      // Enable colors for tail -f
	}

	// Custom timestamp formatter showing elapsed time
	writer.FormatTimestamp = func(i interface{}) string {
		elapsed := time.Since(startTime)
		return fmt.Sprintf("\x1b[90m[+%s]\x1b[0m",
			elapsed.Round(time.Millisecond))
	}

	// Create logger with timestamp
	log.Logger = zerolog.New(writer).With().Timestamp().Logger()
}
