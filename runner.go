package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	logger.Info().Str("target", build_output).Msg("Build output")
	p := process.New(build_output)
	// Avoid infinite recursion when we self-host
	if base == "flogo" {
		logger.Info().Msg("Refusing to infinitely recurse on flogo")
		r.onStart()
		r.onOutput(p)
		return nil
	}
	sub_exit := p.OnExit.Subscribe()
	sub_start := p.OnStart.Subscribe()
	//sub_stderr := p.OnStderr.Subscribe()
	//sub_stdout := p.OnStdout.Subscribe()
	sub_output := p.OnOutput.Subscribe()
	defer sub_exit.Close()
	defer sub_start.Close()
	//defer sub_stderr.Close()
	//defer sub_stdout.Close()
	defer sub_output.Close()
	// Start runner by starting the command, if we can
	err = p.Start(ctx)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info().Err(err).Msg("Runner process does not exist, waiting for it to be built")
			r.onWaiting()
		}
		logger.Warn().Err(err).Msg("failed to start runner process")
	}
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
		/*
			case b := <-sub_stderr.C:
				log.Debug().Bytes("b", b).Msg("subprocess stderr")
				r.onOutput(p)
			case b := <-sub_stdout.C:
				log.Debug().Bytes("b", b).Msg("subprocess stdout")
				r.onOutput(p)
		*/
		case b := <-sub_output.C:
			log.Debug().Bytes("b", b).Msg("subprocess output")
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
			output:   p.Output.Bytes(),
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
