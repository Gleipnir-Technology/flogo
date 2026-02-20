package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type EventBuilderType int

const (
	EventBuildFailure EventBuilderType = iota
	EventBuildStart
	EventBuildSuccess
)

type EventBuilder struct {
	Message string
	Type    EventBuilderType
}
type Builder struct {
	Debounce time.Duration
	OnDeath  chan<- error
	OnEvent  chan<- EventBuilder
	Target   string
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

// BuildProject builds the Go project
func (b Builder) BuildProject(ctx context.Context) debouncedFunc {
	return func() {
		logger := log.Ctx(ctx)
		cmd := exec.CommandContext(ctx, "go", "build", ".")
		cmd.Dir = b.Target
		stderr, err := cmd.StderrPipe()
		if err != nil {
			logger.Error().Msg("no stderr")
			b.onBuildFailure("no stderr")
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error().Msg("no stdout")
			b.onBuildFailure("no stdout")
			return
		}
		err = cmd.Start()
		if err != nil {
			logger.Error().Msg("no start")
			b.onBuildFailure(fmt.Sprintf("failed to start 'go build .' in %s: %+v", b.Target, err))
			return
		}
		logger.Info().Msg("Build start")
		b.onBuildStart("go build .")
		stderr_b, err := io.ReadAll(stderr)
		if err != nil {
			b.onBuildFailure(fmt.Sprintf("Failed to read stderr: %+v", err))
			return
		}
		logger.Info().Msg("read stderr")
		stdout_b, err := io.ReadAll(stdout)
		if err != nil {
			b.onBuildFailure(fmt.Sprintf("Failed to read stdout: %+v", err))
			return
		}
		logger.Info().Msg("read stdout")
		err = cmd.Wait()
		if err != nil {
			b.onBuildFailure(string(stderr_b))
			return
		}
		logger.Info().Bytes("stdout", stdout_b).Bytes("stderr", stderr_b).Msg("build complete")
		b.onBuildSuccess(string(stdout_b))
	}
}

func (b Builder) onBuildFailure(f string) {
	b.OnEvent <- EventBuilder{
		Message: f,
		Type:    EventBuildFailure,
	}
}
func (b Builder) onBuildStart(m string) {
	b.OnEvent <- EventBuilder{
		Message: m,
		Type:    EventBuildStart,
	}
}
func (b Builder) onBuildSuccess(m string) {
	b.OnEvent <- EventBuilder{
		Message: m,
		Type:    EventBuildSuccess,
	}
}
func (b Builder) onError(err error) {
	log.Error().Err(err).Msg("HANDLE THIS")
}

// determineBuildOutputName determines the build output name from the go.mod file
func determineBuildOutputAbs(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("Failed to get abs path: %w", err)
	}
	// Check if we're in a Go module
	go_mod_path := filepath.Join(".", "go.mod")
	if _, err := os.Stat(go_mod_path); os.IsNotExist(err) {
		return "", fmt.Errorf("no go.mod file found, not in a Go module")
	}

	// Use go list to get the module name
	cmd := exec.Command("go", "list", "-f", "{{.Name}}", "./"+target)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Failed to run 'go list -m': %w", err)
	}

	// Extract the last part of the module path
	module := strings.TrimSpace(string(output))
	if module == "main" {
		// For the special name "main" we use the parent directory name
		// since that's what 'go build' will do
		base := filepath.Base(abs)
		// We then re-add that base to get the full abs path to the build output
		full := filepath.Join(abs, base)
		return full, nil
	}
	return "", fmt.Errorf("Not sure what to do with '%s': this isn't implemented yet", module)
}
