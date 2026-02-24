package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/rs/zerolog/log"
)

type uiFlat struct {
	onEvents chan Event
}

func newUIFlat() (*uiFlat, error) {
	return &uiFlat{
		onEvents: make(chan Event),
	}, nil
}
func (u *uiFlat) Close() {}
func (u *uiFlat) Events() <-chan Event {
	return u.onEvents
}
func (u *uiFlat) Run(ctx context.Context, chanOnEvent chan<- Event, chanNewState <-chan *state.Flogo) error {
	logger := log.Ctx(ctx).With().Caller().Logger()
	for {
		select {
		case <-ctx.Done():
			logger.Debug().Msg("context ended, exiting UI")
			return nil
		case s := <-chanNewState:
			u.dump(s)
		}
	}
}
func (u *uiFlat) dump(s *state.Flogo) {
	output := "waiting..."
	if s.Builder.Status != state.StatusBuilderOK {
		if s.Builder.BuildCurrent != nil && len(s.Builder.BuildCurrent.Output) > 0 {
			output = string(s.Builder.BuildCurrent.Output)
		} else if s.Builder.BuildPrevious != nil && len(s.Builder.BuildPrevious.Output) > 0 {
			output = string(s.Builder.BuildPrevious.Output)
		} else {
			output = "no build output"
		}
	} else {
		if s.Runner.RunCurrent != nil && len(s.Runner.RunCurrent.Output) > 0 {
			output = string(s.Runner.RunCurrent.Output)
		} else if s.Runner.RunPrevious != nil && len(s.Runner.RunPrevious.Output) > 0 {
			output = string(s.Runner.RunPrevious.Output)
		} else {
			output = "no run output"
		}
	}
	output = strings.TrimSpace(output)
	fmt.Printf("builder %s\trunner %s\t%s\n",
		state.StatusStringBuilder(s.Builder.Status),
		state.StatusStringRunner(s.Runner.Status),
		output,
	)
}
