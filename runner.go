package main

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"
)

type EventRunnerType int

const (
	EventRunnerStart EventRunnerType = iota
	EventRunnerStop
	EventRunnerStdout
	EventRunnerStderr
)

type EventRunner struct {
	Message string
	Type    EventRunnerType
}
type Runner struct {
	DoRestart <-chan struct{}
	OnEvent   chan<- EventRunner
	Target    string
}

func (r Runner) Run(ctx context.Context) {
	logger := log.Ctx(ctx)
	build_name, err := determineBuildOutputName(r.Target)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to determine build output name")
		return
	}
	// Avoid infinite recursion when we self-host
	if build_name == "flogo" {
		logger.Info().Msg("Refusing to infinitely recurse on flogo")
		r.OnEvent <- EventRunner{
			Message: "",
			Type:    EventRunnerStart,
		}
		r.OnEvent <- EventRunner{
			Message: "no recursing!",
			Type:    EventRunnerStdout,
		}
	}
	// Create the command
	cmd := exec.Command(build_name)
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
		logger.Error().Err(err).Msg("Failed to start")
		return
	}

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			fmt.Println("stdout:", scanner.Text())
		}
	}()

	// Read stderr line by line
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			fmt.Println("stderr:", stderrScanner.Text())
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		logger.Error().Err(err).Msg("Failed to start")
		return
	}
}
