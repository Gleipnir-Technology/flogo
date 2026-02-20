package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

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
	Process *stateProcess
	Type    EventRunnerType
}
type Runner struct {
	Parent

	DoRestart <-chan struct{}
	OnDeath   chan<- error
	OnEvent   chan<- EventRunner
	Target    string

	buildOutput string
}

func (r *Runner) Run(ctx context.Context) {
	logger := log.Ctx(ctx)
	var err error
	r.buildOutput, err = determineBuildOutputAbs(r.Target)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to determine build output name")
		r.OnDeath <- fmt.Errorf("Failed to determine build output name")
		return
	}
	base := filepath.Base(r.buildOutput)
	// Avoid infinite recursion when we self-host
	if base == "flogo" {
		logger.Info().Msg("Refusing to infinitely recurse on flogo")
		r.onStart()
		r.onStdout([]byte("no flogo recursing."))
		return
	}
	logger.Info().Str("build", r.buildOutput).Msg("Build output")
	chan_restart := make(chan struct{})
	go r.supervise(ctx, chan_restart)
	for {
		select {
		case <-ctx.Done():
			if r.Child != nil {
				r.Child.Process.Signal(syscall.SIGINT)
			}
			return
		case <-r.DoRestart:
			if r.Child != nil {
				r.Child.Process.Signal(syscall.SIGINT)
			}
			chan_restart <- struct{}{}
		}
	}
}

func (r *Runner) restart(ctx context.Context) {
	logger := log.Ctx(ctx)
	r.Stdout.Reset()
	r.Stderr.Reset()
	if _, err := os.Stat(r.buildOutput); os.IsNotExist(err) {
		logger.Info().Str("build_output", r.buildOutput).Msg("Build output doesn't exist")
		r.onStart()
		r.onStdout([]byte("refusing flogo recursion"))
		return
	}
	// Create the command
	r.Child = exec.Command(r.buildOutput)
	r.Child.Dir = r.Target

	// Get a pipe for stdout
	stdout, err := r.Child.StdoutPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stdout pipe")
		r.Child = nil
		return
	}

	// get stderr too
	stderr, err := r.Child.StderrPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stderr pipe")
		r.Child = nil
		return
	}

	// Start the command (non-blocking)
	if err := r.Child.Start(); err != nil {
		logger.Error().Err(err).Str("build_output", r.buildOutput).Msg("Failed to start")
		r.Child = nil
		return
	}
	r.onStart()

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			b := scanner.Bytes()
			r.onStdout(b)
		}
	}()

	// Read stderr line by line
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			b := stderrScanner.Bytes()
			r.onStderr(b)
		}
	}()

	// Wait for the command to finish
	if e := r.Child.Wait(); e != nil {
		if ex, ok := err.(*exec.ExitError); ok {
			i := ex.ExitCode()
			r.onFailure(&i, []byte{}, ex.Stderr)
		} else {
			r.onFailure(nil, []byte(err.Error()), []byte{})
		}
		r.Child = nil
		return
	}
	r.onSuccess()
	r.Child = nil
}
func (r *Runner) supervise(ctx context.Context, chan_restart <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-chan_restart:
			r.restart(ctx)
		}
	}
}
func (r *Runner) onFailure(exit_code *int, stdout []byte, stderr []byte) {
	r.OnEvent <- EventRunner{
		Process: &stateProcess{
			exitCode: exit_code,
			stdout:   stdout,
			stderr:   stderr,
		},
		Type: EventRunnerStart,
	}
}
func (r *Runner) onStart() {
	r.OnEvent <- EventRunner{
		Process: nil,
		Type:    EventRunnerStart,
	}
}
func (r *Runner) onSuccess() {
	i := 0
	r.OnEvent <- EventRunner{
		Process: &stateProcess{
			exitCode: &i,
			stdout:   r.Stdout.Bytes(),
			stderr:   r.Stderr.Bytes(),
		},
		Type: EventRunnerStopOK,
	}
}
