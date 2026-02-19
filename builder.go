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
	Debounce time.Duration
	OnDeath  chan<- error
	ToBuild  <-chan struct{}
}

func (b Builder) Run(ctx context.Context) {
	logger := log.Ctx(ctx)
	debounce := newDebounce(ctx, b.Debounce)
	for {
		select {
		case <-b.ToBuild:
			debounce(b.BuildProject(ctx))
		case <-ctx.Done():
			logger.Info().Msg("Shutdown builder")
			return
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
func (b Builder) BuildProject(ctx context.Context) debouncedFunc {
	return func() {
		logger := log.Ctx(ctx)
		cmd := exec.CommandContext(ctx, "go", "build", ".")
		output, err := cmd.CombinedOutput()
		if err != nil {
			b.onError(fmt.Errorf("go build cmd: %w", err))
		}
		outputStr := string(output)
		logger.Info().Str("out", outputStr).Msg("build complete")
	}
}

func (b Builder) onError(err error) {
	log.Error().Err(err).Msg("HANDLE THIS")
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
