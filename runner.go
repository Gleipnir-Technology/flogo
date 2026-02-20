package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type EventRunnerType int

const (
	EventRunnerStart EventRunnerType = iota
	EventRunnerStopOK
	EventRunnerStopErr
	EventRunnerStdout
	EventRunnerStderr
	EventRunnerWaiting
)

type EventRunner struct {
	Message string
	Type    EventRunnerType
}
type Runner struct {
	DoRestart <-chan struct{}
	OnEvent   chan<- EventRunner
	Target    string
	stdout    strings.Builder
	stderr    strings.Builder
}

func (r *Runner) Run(ctx context.Context) {
	logger := log.Ctx(ctx)
	build_output, err := determineBuildOutputAbs(r.Target)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to determine build output name")
		return
	}
	logger.Info().Str("build", build_output).Msg("Build output")
	// Avoid infinite recursion when we self-host
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("closing runner")
			return
		case <-r.DoRestart:
			go r.Restart(ctx, build_output)
		}
	}
}

func (r *Runner) Restart(ctx context.Context, build_output string) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("runner restart")
	r.stdout.Reset()
	r.stderr.Reset()
	base := filepath.Base(build_output)
	if base == "flogo" {
		logger.Info().Msg("Refusing to infinitely recurse on flogo")
		r.OnEvent <- EventRunner{
			Message: "",
			Type:    EventRunnerStart,
		}
		r.OnEvent <- EventRunner{
			Message: "no recursing!",
			Type:    EventRunnerStdout,
		}
		return
	}
	if _, err := os.Stat(build_output); os.IsNotExist(err) {
		logger.Info().Str("build_output", build_output).Msg("Build output doesn't exist")
		r.OnEvent <- EventRunner{
			Message: "",
			Type:    EventRunnerWaiting,
		}
		return
	}
	// Create the command
	cmd := exec.Command(build_output)
	cmd.Dir = r.Target

	// Get a pipe for stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stdout pipe")
		return
	}

	// Optionally get stderr too
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stderr pipe")
		return
	}

	// Start the command (non-blocking)
	if err := cmd.Start(); err != nil {
		logger.Error().Err(err).Str("build_output", build_output).Msg("Failed to start")
		return
	}
	r.OnEvent <- EventRunner{
		Message: "",
		Type:    EventRunnerStart,
	}

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			text := scanner.Text()
			r.onStdout(text)
		}
	}()

	// Read stderr line by line
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			text := stderrScanner.Text()
			r.onStderr(text)
		}
	}()

	// Wait for the command to finish
	if e := cmd.Wait(); e != nil {
		r.OnEvent <- EventRunner{
			Message: e.Error(),
			Type:    EventRunnerStopErr,
		}
	}
	r.OnEvent <- EventRunner{
		Message: "",
		Type:    EventRunnerStopOK,
	}
	log.Debug().Msg("exiting runner restart")
}

func (r *Runner) onStdout(s string) {
	r.stdout.WriteString(s + "\n")
	r.OnEvent <- EventRunner{
		Message: r.stdout.String(),
		Type:    EventRunnerStdout,
	}
}
func (r *Runner) onStderr(s string) {
	r.stderr.WriteString(s + "\n")
	r.OnEvent <- EventRunner{
		Message: r.stderr.String(),
		Type:    EventRunnerStderr,
	}
}
