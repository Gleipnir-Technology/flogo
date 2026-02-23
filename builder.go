package main

import (
	"context"
	"os"
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
	OnEvent  chan<- EventBuilder
	Target   string
	ToBuild  <-chan struct{}
}

func (b Builder) Run(ctx context.Context) error {
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
			return nil
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
				log.Info().Msg("rebuild.")
				err := p.Start(ctx)
				if err != nil {
					log.Error().Err(err).Msg("failed to start")
				}
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
