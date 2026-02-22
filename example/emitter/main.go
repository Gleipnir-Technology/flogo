package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	// Set up ticker for regular output
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	counter := 0

	for {
		select {
		case <-ticker.C:
			counter++
			if counter%2 == 0 {
				fmt.Printf("Counter: out %d\n", counter)
			} else {
				fmt.Fprintf(os.Stderr, "Counter: err %d\n", counter)
			}

		case <-sigChan:
			fmt.Println("Received SIGINT, shutting down...")
			time.Sleep(1 * time.Second)
			fmt.Println("Exiting.")
			os.Exit(0)
		}
	}
}
