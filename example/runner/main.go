package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"syscall"
	"time"
)

type LogLine struct {
	Source string // "stdout" or "stderr"
	Line   string
}

// ProcessManager handles lifecycle of a subprocess
type ProcessManager struct {
	command    string
	args       []string
	outputChan chan<- LogLine
	restartCh  <-chan struct{}
}

func NewProcessManager(command string, args []string, outputChan chan<- LogLine, restartCh <-chan struct{}) *ProcessManager {
	return &ProcessManager{
		command:    command,
		args:       args,
		outputChan: outputChan,
		restartCh:  restartCh,
	}
}

// Run starts the process and handles restarts
func (pm *ProcessManager) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down")
			return ctx.Err()
		default:
			log.Printf("Starting process: %s %v\n", pm.command, pm.args)
			if err := pm.runOnce(ctx); err != nil {
				log.Printf("Process exited with error: %v\n", err)
			}
			log.Println("Process stopped, waiting for restart signal or context cancel")

			// Wait for restart signal or context cancellation
			select {
			case <-pm.restartCh:
				log.Println("Restart signal received, restarting process...")
				continue
			case <-ctx.Done():
				log.Println("Context cancelled during restart wait")
				return ctx.Err()
			}
		}
	}
}

// runOnce starts the process once and handles its lifecycle
func (pm *ProcessManager) runOnce(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, pm.command, pm.args...)

	// Get pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	log.Printf("Process started with PID: %d\n", cmd.Process.Pid)

	// Stream stdout and stderr to channel
	done := make(chan struct{})
	go pm.streamOutput(stdout, "stdout", done)
	go pm.streamOutput(stderr, "stderr", done)

	// Wait for either:
	// 1. Context cancellation (graceful shutdown requested)
	// 2. Process exit
	// 3. Restart signal
	processDone := make(chan error, 1)
	go func() {
		processDone <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Graceful shutdown requested
		log.Println("Shutdown requested, sending SIGTERM to process")
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Failed to send SIGTERM: %v, killing process\n", err)
			cmd.Process.Kill()
		}

		// Wait for process to exit (with timeout)
		shutdownTimeout := time.NewTimer(10 * time.Second)
		defer shutdownTimeout.Stop()

		select {
		case err := <-processDone:
			log.Println("Process gracefully terminated")
			// Wait for output streams to finish
			<-done
			<-done
			return err
		case <-shutdownTimeout.C:
			log.Println("Shutdown timeout, killing process")
			cmd.Process.Kill()
			<-processDone
			<-done
			<-done
			return fmt.Errorf("process killed after shutdown timeout")
		}

	case <-pm.restartCh:
		// Restart requested while process is running
		log.Println("Restart signal received, shutting down current process")
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Failed to send SIGTERM: %v, killing process\n", err)
			cmd.Process.Kill()
		}
		err := <-processDone
		<-done
		<-done
		return err

	case err := <-processDone:
		// Process exited on its own
		log.Println("Process exited")
		<-done
		<-done
		return err
	}
}

// streamOutput reads from a pipe and sends lines to the output channel
func (pm *ProcessManager) streamOutput(pipe io.ReadCloser, source string, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		pm.outputChan <- LogLine{
			Source: source,
			Line:   scanner.Text(),
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from %s: %v\n", source, err)
	}
}

// Example usage
func main() {
	// Create channels
	outputChan := make(chan LogLine, 100)
	restartCh := make(chan struct{})

	// Start a goroutine to consume output
	go func() {
		for line := range outputChan {
			fmt.Printf("[%s] %s\n", line.Source, line.Line)
		}
	}()

	// Example: simulate restart signal after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		log.Println("Sending restart signal...")
		restartCh <- struct{}{}

		// Send another restart after 5 more seconds
		time.Sleep(5 * time.Second)
		log.Println("Sending another restart signal...")
		restartCh <- struct{}{}
	}()

	// Create process manager
	// Replace with your actual command, e.g., "your-long-running-app"
	pm := NewProcessManager("sh", []string{"-c", "while true; do echo 'Hello from subprocess'; sleep 1; done"}, outputChan, restartCh)

	// Run with context (can be cancelled for graceful shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := pm.Run(ctx); err != nil && err != context.DeadlineExceeded {
		log.Fatalf("Process manager error: %v\n", err)
	}

	close(outputChan)
	log.Println("Shutdown complete")
}
