package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime/debug"

	"github.com/Gleipnir-Technology/flogo/ui"
	"github.com/rs/zerolog/log"
)

func main() {
	var err error
	ui_type := os.Getenv("FLOGO_UI")

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
	logger := setupLogging(file)

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
	var u ui.UI
	switch ui_type {
	case "", "tcell":
		u, err = ui.NewTUI(*target, *upstreamURL)
	case "flat":
		u, err = ui.NewFlat(*target, *upstreamURL)
	default:
		fmt.Printf("Unrecognized FLOGO_UI '%s'\n", ui_type)
		os.Exit(3)
	}
	if err != nil {
		fmt.Printf("Failed to create UI: %+v\n", err)
		os.Exit(4)
	}
	// Handle keyboard input
	//c := make(chan os.Signal, 1)
	//signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Keep track of the state of everything
	mgr := newFlogoStateManager()

	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Bytes("stack", debug.Stack()).
				Msg("Application panicked")
			// Also write to stderr file
			fmt.Fprintf(os.Stderr, "PANIC: %v\n%s\n", r, debug.Stack())
		}
	}()
	err = mgr.Run(logger, u, bind, *target)
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
}
func reload() {
	log.Info().Msg("fake reload")
}
