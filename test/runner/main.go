package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Gleipnir-Technology/flogo/process"
)

// Example usage of the process class
func main() {
	process := process.New("../../example/emitter/emitter")
	ctx := context.Background()
	process.Start(ctx)
	count := 0
	timer := time.After(5 * time.Second)
	for {
		select {
		case <-process.OnExit():
			fmt.Println("Child exited")
			process.Start(ctx)
			if count < 3 {
				count = count + 1
			} else {
				os.Exit(0)
			}
		case <-process.OnStart():
			fmt.Println("Child started")
		case b := <-process.OnStderr():
			fmt.Printf("child stderr: %s\n", string(b))
		case b := <-process.OnStdout():
			fmt.Printf("child stdout: %s\n", string(b))
		case <-timer:
			fmt.Printf("timer elapsed, count %d\n", count)
			process.SignalInterrupt()
			timer = time.After(5 * time.Second)
		}
	}
}
