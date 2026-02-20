package main

import (
	"bufio"
	"context"
	"fmt"
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
	EventBuildOutput
	EventBuildStart
	EventBuildSuccess
)

type EventBuilder struct {
	Process *stateProcess
	Type    EventBuilderType
}
type Builder struct {
	Parent
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
func (bld Builder) BuildProject(ctx context.Context) debouncedFunc {
	return func() {
		logger := log.Ctx(ctx)
		cmd := exec.CommandContext(ctx, "go", "build", ".")
		cmd.Dir = bld.Target
		stderr, err := cmd.StderrPipe()
		if err != nil {
			logger.Error().Msg("no stderr")
			bld.onBuildFailure(nil, []byte{}, []byte{})
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error().Msg("no stdout")
			bld.onBuildFailure(nil, []byte{}, []byte{})
			return
		}
		err = cmd.Start()
		if err != nil {
			logger.Error().Msg("no start")
			bld.onBuildFailure(nil, []byte{}, []byte(fmt.Sprintf("failed to start 'go build .' in %s: %+v", bld.Target, err)))
			return
		}
		scanner := bufio.NewScanner(stdout)
		go func() {
			for scanner.Scan() {
				b := scanner.Bytes()
				bld.onStdout(b)
			}
		}()

		// Read stderr line by line
		stderrScanner := bufio.NewScanner(stderr)
		go func() {
			for stderrScanner.Scan() {
				b := stderrScanner.Bytes()
				bld.onStderr(b)
			}
		}()

		logger.Info().Msg("Build start")
		bld.onBuildStart("go build .")

		if err = cmd.Wait(); err != nil {
			if ex, ok := err.(*exec.ExitError); ok {
				i := ex.ExitCode()
				bld.onBuildFailure(&i, []byte{}, ex.Stderr)
			} else {
				bld.onBuildFailure(nil, []byte(err.Error()), []byte{})
			}
			return
		}
		//logger.Info().Bytes("stdout", stdout_b).Bytes("stderr", stderr_b).Msg("build complete")
		bld.onBuildSuccess()
	}
}

func (b Builder) onBuildOutput(stdout []byte, stderr []byte) {
	b.OnEvent <- EventBuilder{
		Process: &stateProcess{
			exitCode: nil,
			stdout:   stdout,
			stderr:   stderr,
		},
		Type: EventBuildOutput,
	}
}
func (b Builder) onBuildFailure(exit_code *int, stdout []byte, stderr []byte) {
	b.OnEvent <- EventBuilder{
		Process: &stateProcess{
			exitCode: exit_code,
			stdout:   stdout,
			stderr:   stderr,
		},
		Type: EventBuildFailure,
	}
}
func (b Builder) onBuildStart(m string) {
	b.OnEvent <- EventBuilder{
		Process: nil,
		Type:    EventBuildStart,
	}
}
func (b Builder) onBuildSuccess() {
	i := 0
	b.OnEvent <- EventBuilder{
		Process: &stateProcess{
			exitCode: &i,
			stdout:   b.Stdout.Bytes(),
			stderr:   b.Stderr.Bytes(),
		},
		Type: EventBuildSuccess,
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
	args := []string{"list", "-f", "{{.Name}}", "./" + target}
	cmd := exec.Command("go", args...)
	output, err := cmd.Output()
	if err != nil {
		full_cmd := "go " + strings.Join(args, " ")
		if ex, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Failed to run '%s': %s, %w", full_cmd, string(ex.Stderr), ex)
		} else {
			return "", fmt.Errorf("Failed to run '%s': %w", full_cmd, err)
		}
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
