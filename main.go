package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"sync"
)

var (
	lastBuildOutput  string
	lastBuildSuccess bool
	uiMutex          sync.Mutex
)

func main() {
	var err error
	initUI()
	defer screen.Fini()

	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:8080" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		log.Fatalf("Invalid FLOGO_UPSTREAM URL: %v", err)
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the UI in a goroutine
	go runUI(ctx)

	// Start the web server
	go startServer()

	// Start the file watcher
	go setupWatcher()

	// Handle keyboard input
	for {
		handleInput()
	}
}
