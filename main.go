package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/exec"
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
func buildProject() {
	uiMutex.Lock()
	isCompiling = true
	uiMutex.Unlock()

	log.Println("Building project...")

	cmd := exec.Command("go", "build", ".")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	uiMutex.Lock()
	isCompiling = false
	lastBuildOutput = outputStr
	lastBuildSuccess = (err == nil)
	uiMutex.Unlock()

	if err != nil {
		log.Println("Build failed:")
		log.Println(outputStr)
	} else {
		log.Println("Build succeeded!")
	}
}
