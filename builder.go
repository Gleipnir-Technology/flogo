package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gleipnir-Technology/flogo/process"
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
	Debounce time.Duration
	OnDeath  chan<- error
	OnEvent  chan<- EventBuilder
	Target   string
	ToBuild  <-chan struct{}
}

func (b Builder) Run(ctx context.Context) {
	logger := log.Ctx(ctx)
	debounce := newDebounce(ctx, b.Debounce)
	p := process.New("go", "build", ".")
	p.SetDir(b.Target)
	sub_exit := p.OnExit.Subscribe()
	sub_output := p.OnOutput.Subscribe()
	sub_start := p.OnStart.Subscribe()
	defer sub_exit.Close()
	defer sub_output.Close()
	defer sub_start.Close()
	err := p.Start(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to start builder process")
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Shutdown builder")
			return
		case state := <-sub_exit.C:
			log.Info().Msg("Runner's process exited")
			b.onExit(p, state)
		case <-sub_start.C:
			log.Info().Msg("Runner's process started")
			b.onStart()
		case buf := <-sub_output.C:
			log.Debug().Bytes("b", buf).Msg("subprocess output")
			b.onOutput(p)
		case <-b.ToBuild:
			debounce(func() {
				p.Start(ctx)
			})
		}
	}
}

func (b Builder) onOutput(p *process.Process) {
	b.OnEvent <- EventBuilder{
		Process: &stateProcess{
			exitCode: nil,
			output:   p.Output.Bytes(),
			stderr:   p.Stderr.Bytes(),
			stdout:   p.Stdout.Bytes(),
		},
		Type: EventBuildOutput,
	}
}
func (b Builder) onExit(p *process.Process, state *os.ProcessState) {
	var t EventBuilderType
	i := state.ExitCode()
	if i == 0 {
		t = EventBuildSuccess
	} else {
		t = EventBuildFailure
	}
	b.OnEvent <- EventBuilder{
		Process: &stateProcess{
			exitCode: &i,
			stderr:   p.Stderr.Bytes(),
			stdout:   p.Stdout.Bytes(),
		},
		Type: t,
	}
}
func (b Builder) onStart() {
	b.OnEvent <- EventBuilder{
		Process: nil,
		Type:    EventBuildStart,
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
