package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CompilerManager handles building the project and notifying the subprocess manager
type CompilerManager struct {
	mutex            sync.Mutex
	isCompiling      bool
	lastBuildOutput  string
	lastBuildSuccess bool
	lastBuildTime    time.Time
	compileDone      chan struct{}
	subprocessMgr    *SubprocessManager
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewCompilerManager creates a new compiler manager
func NewCompilerManager() *CompilerManager {
	ctx, cancel := context.WithCancel(context.Background())
	compileDone := make(chan struct{})

	return &CompilerManager{
		compileDone: compileDone,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetSubprocessManager sets the subprocess manager reference
func (cm *CompilerManager) SetSubprocessManager(subprocessMgr *SubprocessManager) {
	cm.subprocessMgr = subprocessMgr
}

// GetCompileDoneChannel returns the channel that signals when compilation is complete
func (cm *CompilerManager) GetCompileDoneChannel() <-chan struct{} {
	return cm.compileDone
}

// BuildProject builds the Go project
func (cm *CompilerManager) BuildProject() {
	cm.mutex.Lock()
	// Don't start a new build if one is already in progress
	if cm.isCompiling {
		cm.mutex.Unlock()
		return
	}
	cm.isCompiling = true
	cm.mutex.Unlock()

	log.Println("Building project...")

	cmd := exec.CommandContext(cm.ctx, "go", "build", ".")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	cm.mutex.Lock()
	cm.isCompiling = false
	cm.lastBuildOutput = outputStr
	cm.lastBuildSuccess = (err == nil)
	cm.lastBuildTime = time.Now()

	// Create a new completion channel for future builds
	close(cm.compileDone)
	cm.compileDone = make(chan struct{})

	buildSuccess := cm.lastBuildSuccess
	cm.mutex.Unlock()

	if err != nil {
		log.Println("Build failed:")
		log.Println(outputStr)
	} else {
		log.Println("Build succeeded!")

		// If the subprocess manager exists and build succeeded, restart the process
		if cm.subprocessMgr != nil && buildSuccess {
			go func() {
				// Give the filesystem a moment to finalize the file writes
				time.Sleep(100 * time.Millisecond)

				log.Println("Restarting child process with new build...")
				err := cm.subprocessMgr.Restart()
				if err != nil {
					log.Printf("Failed to restart child process: %v", err)
				}
			}()
		}
	}
}

// TriggerBuild triggers a new build asynchronously
func (cm *CompilerManager) TriggerBuild() {
	go cm.BuildProject()
}

// IsCompiling returns whether a compilation is in progress
func (cm *CompilerManager) IsCompiling() bool {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.isCompiling
}

// GetLastBuildInfo returns information about the last build
func (cm *CompilerManager) GetLastBuildInfo() (string, bool, time.Time) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.lastBuildOutput, cm.lastBuildSuccess, cm.lastBuildTime
}

// Cleanup performs cleanup operations
func (cm *CompilerManager) Cleanup() {
	cm.cancel()
}

// determineBuildOutputName determines the build output name from the go.mod file
func determineBuildOutputName(target string) (string, error) {
	// Check if we're in a Go module
	go_mod_path := filepath.Join(target, "go.mod")
	if _, err := os.Stat(go_mod_path); os.IsNotExist(err) {
		return "", fmt.Errorf("no go.mod file found, not in a Go module")
	}

	// Use go list to get the module name
	cmd := exec.Command("go", "list", "-m")
	cmd.Dir = target
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Failed to run 'go list -m': %w", err)
	}
	// Extract the last part of the module path which is typically the project name
	modulePath := strings.TrimSpace(string(output))
	parts := strings.Split(modulePath, "/")
	return parts[len(parts)-1], nil
}
