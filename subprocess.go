package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// SubprocessManager handles the lifecycle of the child application process
type SubprocessManager struct {
	cmd            *exec.Cmd
	buildName      string
	mutex          sync.Mutex
	isRunning      bool
	compileDone    <-chan struct{} // Signal channel from compiler
	processStarted chan struct{}   // Signal when process starts
	processExited  chan error      // Signal when process exits
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewSubprocessManager creates a new subprocess manager
func NewSubprocessManager(compileDone <-chan struct{}) (*SubprocessManager, error) {
	buildName, err := determineBuildOutputName()
	if err != nil {
		return nil, fmt.Errorf("failed to determine build output name: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SubprocessManager{
		buildName:      buildName,
		compileDone:    compileDone,
		processStarted: make(chan struct{}),
		processExited:  make(chan error, 1), // Buffered to avoid blocking
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Start attempts to start the child process
func (sm *SubprocessManager) Start() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.isRunning {
		return fmt.Errorf("process already running")
	}

	// Check if build output exists
	buildPath := filepath.Join(".", sm.buildName)
	if _, err := os.Stat(buildPath); os.IsNotExist(err) {
		log.Printf("Build output %s not found, waiting for compilation to complete...", buildPath)
		return sm.waitForCompileThenStart()
	}

	return sm.startProcess()
}

// waitForCompileThenStart waits for compilation to complete before starting the process
func (sm *SubprocessManager) waitForCompileThenStart() error {
	go func() {
		select {
		case <-sm.compileDone:
			log.Println("Compilation completed, starting process...")
			sm.mutex.Lock()
			err := sm.startProcess()
			sm.mutex.Unlock()

			if err != nil {
				log.Printf("Failed to start process after compilation: %v", err)
			}
		case <-sm.ctx.Done():
			return
		}
	}()
	return nil
}

// startProcess starts the child process
func (sm *SubprocessManager) startProcess() error {
	buildPath := filepath.Join(".", sm.buildName)
	if _, err := os.Stat(buildPath); os.IsNotExist(err) {
		return fmt.Errorf("build output %s not found", buildPath)
	}

	// Start the process with the current directory set to the project root
	sm.cmd = exec.Command(buildPath)
	sm.cmd.Stdout = os.Stdout
	sm.cmd.Stderr = os.Stderr
	sm.cmd.Env = os.Environ()

	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	sm.isRunning = true

	// Notify listeners that process started
	close(sm.processStarted)
	sm.processStarted = make(chan struct{})

	// Monitor the process in a goroutine
	go func() {
		err := sm.cmd.Wait()
		sm.mutex.Lock()
		sm.isRunning = false
		sm.mutex.Unlock()

		select {
		case sm.processExited <- err:
			log.Printf("Child process exited: %v", err)
		default:
			// Channel is full, which means no one is listening
		}
	}()

	log.Printf("Started child process: %s (PID: %d)", sm.buildName, sm.cmd.Process.Pid)
	return nil
}

// Stop stops the running process
func (sm *SubprocessManager) Stop() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.isRunning || sm.cmd == nil || sm.cmd.Process == nil {
		return nil // Nothing to do
	}

	// First try graceful termination
	if err := sm.cmd.Process.Signal(os.Interrupt); err != nil {
		log.Printf("Failed to send interrupt signal: %v, forcing termination", err)
		if err := sm.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for process to exit with timeout
	select {
	case <-sm.processExited:
		// Process exited
	case <-time.After(5 * time.Second):
		log.Println("Process didn't exit gracefully, forcing termination")
		if err := sm.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	sm.isRunning = false
	return nil
}

// Restart stops the running process and starts it again
func (sm *SubprocessManager) Restart() error {
	if err := sm.Stop(); err != nil {
		return err
	}
	return sm.Start()
}

// IsRunning returns whether the process is currently running
func (sm *SubprocessManager) IsRunning() bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.isRunning
}

// Cleanup performs cleanup operations
func (sm *SubprocessManager) Cleanup() {
	sm.cancel()
	sm.Stop()
}
