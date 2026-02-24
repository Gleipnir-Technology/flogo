package main

import (
	"context"
	"os"
	"time"

	"github.com/Gleipnir-Technology/flogo/process"
	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/rs/zerolog"
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
	Process *state.Process
	Type    EventBuilderType
}
type Builder struct {
	Debounce time.Duration
	OnEvent  chan<- EventBuilder
	Target   string
	ToBuild  <-chan struct{}
}

func (b Builder) Run(ctx context.Context) error {
	logger := log.Ctx(ctx).With().Caller().Logger()

	debounce := newDebounce(ctx, b.Debounce)
	p := process.New("go", "build", ".")
	p.SetDir(b.Target)
	sub_event := p.OnEvent.Subscribe()
	logger.Info().Msg("Started builder loop")
	err := p.Start(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to start builder process")
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Shutdown builder")
			return nil
		case evt := <-sub_event.C:
			switch evt.Type {
			case process.EventProcessStop:
				b.onExit(logger, p, evt.ProcessState)
			case process.EventProcessStart:
				b.onStart(logger)
			case process.EventProcessOutput:
				b.onOutput(logger, p, evt.Data)
			default:
				logger.Warn().Msg("unrecognized process event")
			}
		case <-b.ToBuild:
			debounce(func() {
				logger.Info().Msg("rebuild.")
				err := p.Start(ctx)
				if err != nil {
					logger.Error().Err(err).Msg("failed to start")
				}
			})
		}
	}
}

func (b Builder) onOutput(logger zerolog.Logger, p *process.Process, buf []byte) {
	logger.Debug().Bytes("b", buf).Msg("subprocess output")
	b.OnEvent <- EventBuilder{
		Process: &state.Process{
			ExitCode: nil,
			Output:   p.Output.Bytes(),
			Stderr:   p.Stderr.Bytes(),
			Stdout:   p.Stdout.Bytes(),
		},
		Type: EventBuildOutput,
	}
}
func (b Builder) onExit(logger zerolog.Logger, p *process.Process, s *os.ProcessState) {
	logger.Debug().Msg("process exit")
	var t EventBuilderType
	i := s.ExitCode()
	if i == 0 {
		t = EventBuildSuccess
	} else {
		t = EventBuildFailure
	}
	b.OnEvent <- EventBuilder{
		Process: &state.Process{
			ExitCode: &i,
			Output:   p.Output.Bytes(),
			Stderr:   p.Stderr.Bytes(),
			Stdout:   p.Stdout.Bytes(),
		},
		Type: t,
	}
}
func (b Builder) onStart(logger zerolog.Logger) {
	logger.Debug().Msg("process started")
	b.OnEvent <- EventBuilder{
		Process: nil,
		Type:    EventBuildStart,
	}
}
