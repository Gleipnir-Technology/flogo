package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Builder struct {
	ToBuild <-chan struct{}
	OnDeath chan<- error
}

func (b Builder) Run(ctx context.Context) {
	//logger := log.Ctx(ctx)
	for {
		select {
		case _ = <-b.ToBuild:
			err := b.BuildProject(ctx)
			if err != nil {
				b.OnDeath <- fmt.Errorf("Builder death: %w", err)
				return
			}

		}
	}
}

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

// BuildProject builds the Go project
func (b Builder) BuildProject(ctx context.Context) error {
	logger := log.Ctx(ctx)
	cmd := exec.CommandContext(ctx, "go", "build", ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build cmd: %w", err)
	}
	outputStr := string(output)
	logger.Info().Str("out", outputStr).Msg("build")
	return nil
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
