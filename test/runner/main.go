package main

import (
	"context"
	"fmt"
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
	sub_exit := process.OnExit.Subscribe()
	sub_start := process.OnStart.Subscribe()
	sub_stderr := process.OnStderr.Subscribe()
	sub_stdout := process.OnStdout.Subscribe()
	defer sub_exit.Close()
	defer sub_start.Close()
	defer sub_stderr.Close()
	defer sub_stdout.Close()
	for {
		select {
		case <-sub_exit.C:
			fmt.Println("Child exited")
		case <-sub_start.C:
			fmt.Println("Child started")
		case b := <-sub_stderr.C:
			fmt.Printf("child stderr: %s\n", string(b))
		case b := <-sub_stdout.C:
			fmt.Printf("child stdout: %s\n", string(b))
		case <-timer:
			fmt.Printf("timer elapsed, count %d\n", count)
			process.Restart(ctx)
			timer = time.After(5 * time.Second)
		}
	}
}
