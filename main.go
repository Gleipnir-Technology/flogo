package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	lastBuildOutput  string
	lastBuildSuccess bool
	uiMutex          sync.Mutex
)

func main() {
	var err error
	var target = flag.String("target", ".", "The directory containing the go project to build")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
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
	logger := zerolog.New(file).With().Timestamp().Logger()
	log.Logger = logger

	bind := os.Getenv("FLOGO_BIND")
	if bind == "" {
		bind = ":10000" // Default if not specified
	}
	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:8080" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		fmt.Printf("Failed to parse '%s' as a URL: %v\n", upstream, err)
		os.Exit(2)
	}

	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Str("component", "arcgis").Logger().WithContext(context.Background()))
	defer cancel()

	// Create a channel where all the goroutines can signal death
	something_died := make(chan error)

	go runFileWatcher(ctx, something_died, *target)
	// Start the UI in a goroutine
	//go runUI(ctx)

	// Start the subprocess
	/*compile_done := make(chan struct{})
	manager, err := NewSubprocessManager(compile_done, *target)
	if err != nil {
		fmt.Printf("Failed to create subproccess manager: %v\n", err)
		os.Exit(3)
	}
	go manager.Start()

	// Start the web server
	go startServer(bind)

	//initUI()
	//defer screen.Fini()
	// Handle keyboard input
	for {
		handleInput()
	}
	*/
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case death := <-something_died:
		fmt.Printf("Death: %v\n", death)
		cancel()
	case sig := <-c:
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			cancel()
		case syscall.SIGHUP:
			reload()
		}
	}
}
func reload() {
}
