package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Gleipnir-Technology/flogo/process"
	"github.com/rs/zerolog/log"
)

type EventRunnerType int

const (
	EventRunnerOutput EventRunnerType = iota
	EventRunnerStart
	EventRunnerStopOK
	EventRunnerStopErr
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
}

func (r *Runner) Run(ctx context.Context) error {
	logger := log.Ctx(ctx)
	build_output, err := determineBuildOutputAbs(r.Target)
	if err != nil {
		return fmt.Errorf("Failed to determine build output name: %v", err)
	}
	base := filepath.Base(build_output)
	// Avoid infinite recursion when we self-host
	if base == "flogo" {
		logger.Info().Msg("Refusing to infinitely recurse on flogo")
		r.onStart()
		r.onStdout([]byte("no flogo recursing."))
		return nil
	}
	logger.Info().Str("target", build_output).Msg("Build output")
	p := process.New(build_output)
	sub_exit := p.OnExit.Subscribe()
	sub_start := p.OnStart.Subscribe()
	sub_stderr := p.OnStderr.Subscribe()
	sub_stdout := p.OnStdout.Subscribe()
	defer sub_exit.Close()
	defer sub_start.Close()
	defer sub_stderr.Close()
	defer sub_stdout.Close()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context done, exiting runner")
			p.SignalInterrupt()
			return ctx.Err()
		case status := <-sub_exit.C:
			log.Info().Msg("Runner's process exited")
			r.onExit(p, status)
		case <-sub_start.C:
			log.Info().Msg("Runner's process started")
			r.onStart()
		case b := <-sub_stderr.C:
			log.Debug().Bytes("b", b).Msg("subprocess stderr")
			r.onOutput(p)
		case b := <-sub_stdout.C:
			log.Debug().Bytes("b", b).Msg("subprocess stdout")
			r.onOutput(p)
		case <-r.DoRestart:
			log.Info().Msg("Restart signal received, restarting process...")
			err := p.Restart(ctx)
			if err != nil {
				r.OnDeath <- fmt.Errorf("runner restart err: %w", err)
				return nil
			}
		}
	}
}

func (r *Runner) onExit(p *process.Process, state *os.ProcessState) {
	var t EventRunnerType
	i := state.ExitCode()
	if i == 0 {
		t = EventRunnerStopOK
	} else {
		t = EventRunnerStopErr
	}
	r.OnEvent <- EventRunner{
		Process: &stateProcess{
			exitCode: &i,
			stderr:   p.Stderr.Bytes(),
			stdout:   p.Stdout.Bytes(),
		},
		Type: t,
	}
}
func (r *Runner) onOutput(p *process.Process) {
	r.OnEvent <- EventRunner{
		Process: &stateProcess{
			exitCode: nil,
			stderr:   p.Stderr.Bytes(),
			stdout:   p.Stdout.Bytes(),
		},
		Type: EventRunnerOutput,
	}
}
func (r *Runner) onStart() {
	r.OnEvent <- EventRunner{
		Process: nil,
		Type:    EventRunnerStart,
	}
}
func (r *Runner) onWaiting() {
	r.OnEvent <- EventRunner{
		Process: nil,
		Type:    EventRunnerWaiting,
	}
}
