package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gleipnir-Technology/flogo/process"
	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/rs/zerolog"
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
	Process *state.Process
	Type    EventRunnerType
}
type Runner struct {
	DoRestart <-chan struct{}
	OnEvent   chan<- EventRunner
	Target    string
}

func (r *Runner) Run(ctx context.Context) error {
	logger := log.Ctx(ctx).With().Caller().Logger()
	logger.Info().Msg("Started runner loop")
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
		r.onStart(logger)
		r.onOutput(logger, []byte("no flogo recursing"), p)
		return nil
	}
	sub_event := p.OnEvent.Subscribe()
	defer sub_event.Close()
	// Start runner by starting the command, if we can
	err = p.Start(ctx)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info().Err(err).Msg("Runner process does not exist, waiting for it to be built")
			r.onWaiting()
		}
		logger.Warn().Err(err).Msg("failed to start runner process")
	}
	logger.Debug().Msg("Triggered initial runner process")
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Context done, exiting runner")
			p.SignalInterrupt()
			return nil
		case evt := <-sub_event.C:
			switch evt.Type {
			case process.EventProcessOutput:
				go r.onOutput(logger, evt.Data, p)
			case process.EventProcessStart:
				go r.onStart(logger)
			case process.EventProcessStop:
				go r.onExit(logger, p, evt.ProcessState)
			default:
				logger.Warn().Msg("unrecognized process event")
			}
		case <-r.DoRestart:
			logger.Info().Msg("Restart signal received, restarting process...")
			err := p.Restart(ctx)
			if err != nil {
				return fmt.Errorf("runner restart err: %w", err)
			}
		}
	}
}

func (r *Runner) onExit(logger zerolog.Logger, p *process.Process, s *os.ProcessState) {
	var t EventRunnerType
	i := s.ExitCode()
	if i == 0 {
		t = EventRunnerStopOK
	} else {
		t = EventRunnerStopErr
	}
	r.OnEvent <- EventRunner{
		Process: &state.Process{
			ExitCode: &i,
			Output:   p.Output.Bytes(),
			Stderr:   p.Stderr.Bytes(),
			Stdout:   p.Stdout.Bytes(),
		},
		Type: t,
	}
}
func (r *Runner) onOutput(logger zerolog.Logger, b []byte, p *process.Process) {
	logger.Debug().Bytes("b", b).Msg("subprocess output")
	r.OnEvent <- EventRunner{
		Process: &state.Process{
			ExitCode: nil,
			Output:   p.Output.Bytes(),
			Stderr:   p.Stderr.Bytes(),
			Stdout:   p.Stdout.Bytes(),
		},
		Type: EventRunnerOutput,
	}
}
func (r *Runner) onStart(logger zerolog.Logger) {
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
