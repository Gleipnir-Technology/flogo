package main

import (
	"bufio"
	"bytes"
	"context"
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
	Buffer []byte
	Type   EventRunnerType
}
type Runner struct {
	DoRestart <-chan struct{}
	OnDeath   chan<- error
	OnEvent   chan<- EventRunner
	Target    string

	buildOutput string
	child       *exec.Cmd
	stdout      bytes.Buffer
	stderr      bytes.Buffer
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
		r.OnEvent <- EventRunner{
			Buffer: []byte(""),
			Type:   EventRunnerStart,
		}
		r.OnEvent <- EventRunner{
			Buffer: []byte("no recursing!"),
			Type:   EventRunnerStdout,
		}
		return
	}
	logger.Info().Str("build", r.buildOutput).Msg("Build output")
	chan_restart := make(chan struct{})
	go r.parent(ctx, chan_restart)
	for {
		select {
		case <-ctx.Done():
			if r.child != nil {
				r.child.Process.Signal(syscall.SIGINT)
			}
			return
		case <-r.DoRestart:
			if r.child != nil {
				r.child.Process.Signal(syscall.SIGINT)
			}
			chan_restart <- struct{}{}
		}
	}
}

func (r *Runner) onStdout(b []byte) {
	r.stdout.Write(b)
	r.stdout.Write([]byte("\n"))
	r.OnEvent <- EventRunner{
		Buffer: r.stdout.Bytes(),
		Type:   EventRunnerStdout,
	}
}
func (r *Runner) onStderr(b []byte) {
	r.stderr.Write(b)
	r.stderr.Write([]byte("\n"))
	r.OnEvent <- EventRunner{
		Buffer: r.stderr.Bytes(),
		Type:   EventRunnerStderr,
	}
}
func (r *Runner) parent(ctx context.Context, chan_restart <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-chan_restart:
			r.restart(ctx)
		}
	}
}
func (r *Runner) restart(ctx context.Context) {
	logger := log.Ctx(ctx)
	r.stdout.Reset()
	r.stderr.Reset()
	if _, err := os.Stat(r.buildOutput); os.IsNotExist(err) {
		logger.Info().Str("build_output", r.buildOutput).Msg("Build output doesn't exist")
		r.OnEvent <- EventRunner{
			Buffer: []byte(""),
			Type:   EventRunnerWaiting,
		}
		return
	}
	// Create the command
	r.child = exec.Command(r.buildOutput)
	r.child.Dir = r.Target

	// Get a pipe for stdout
	stdout, err := r.child.StdoutPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stdout pipe")
		r.child = nil
		return
	}

	// Optionally get stderr too
	stderr, err := r.child.StderrPipe()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get stderr pipe")
		r.child = nil
		return
	}

	// Start the command (non-blocking)
	if err := r.child.Start(); err != nil {
		logger.Error().Err(err).Str("build_output", r.buildOutput).Msg("Failed to start")
		r.child = nil
		return
	}
	r.OnEvent <- EventRunner{
		Buffer: []byte(""),
		Type:   EventRunnerStart,
	}

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
	if e := r.child.Wait(); e != nil {
		r.OnEvent <- EventRunner{
			Buffer: []byte(e.Error()),
			Type:   EventRunnerStopErr,
		}
		r.child = nil
		return
	}
	r.OnEvent <- EventRunner{
		Buffer: []byte(""),
		Type:   EventRunnerStopOK,
	}
	r.child = nil
}
