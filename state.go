package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/Gleipnir-Technology/flogo/ui"
	"github.com/rs/zerolog"
)

type flogoStateManager struct {
	chanDoBuilder   chan struct{}
	chanDoRunner    chan struct{}
	chanDoUI        chan *state.Flogo
	chanDoWebserver chan *state.Flogo
	chanOnBuilder   chan EventBuilder
	chanOnRunner    chan EventRunner
	chanOnUI        chan ui.Event
	chanOnWatcher   chan struct{}
	isRunning       bool
	state           *state.Flogo
}

func newFlogoStateManager() flogoStateManager {
	return flogoStateManager{
		chanDoRunner:    make(chan struct{}),
		chanDoUI:        make(chan *state.Flogo),
		chanDoWebserver: make(chan *state.Flogo),
		chanOnBuilder:   make(chan EventBuilder),
		chanOnRunner:    make(chan EventRunner),
		chanOnUI:        make(chan ui.Event),
		chanOnWatcher:   make(chan struct{}),
		isRunning:       true,
		state: &state.Flogo{
			Builder: &state.Builder{
				BuildPrevious: nil,
				BuildCurrent:  nil,
				Status:        state.StatusBuilderOK,
			},
			Runner: &state.Runner{
				RunPrevious: nil,
				RunCurrent:  nil,
				Status:      state.StatusRunnerWaiting,
			},
		},
	}
}

func (mgr *flogoStateManager) Run(root_logger zerolog.Logger, u ui.UI, bind string, target string) error {
	// Create a context that we can cancel for signaling all goroutines to clean up
	logger := root_logger.With().Caller().Logger()
	ctx, cancel := context.WithCancel(root_logger.With().Logger().WithContext(context.Background()))
	defer cancel()

	// Create channels for goroutine comms

	watcher := Watcher{
		OnEvent: mgr.chanOnWatcher,
		Target:  target,
	}
	go func() {
		err := watcher.Run(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("watcher died")
			os.Exit(10)
		}
	}()

	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  mgr.chanOnBuilder,
		Target:   target,
		ToBuild:  mgr.chanDoBuilder,
	}
	go func() {
		err := builder.Run(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("builder died")
			os.Exit(11)
		}
	}()

	runner := Runner{
		DoRestart: mgr.chanDoRunner,
		OnEvent:   mgr.chanOnRunner,
		Target:    target,
	}
	go func() {
		err := runner.Run(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("runner died")
			os.Exit(12)
		}
	}()

	// Start the web server
	ws := NewWebserver()
	go func() {
		err := ws.Run(ctx, mgr.chanDoWebserver, bind, *upstreamURL)
		if err != nil {
			logger.Error().Err(err).Msg("webserver died")
			os.Exit(13)
		}
	}()
	// Start the UI in a goroutine
	go func() {
		err := u.Run(ctx, mgr.chanOnUI, mgr.chanDoUI)
		if err != nil {
			logger.Error().Err(err).Msg("ui died")
			os.Exit(14)
		}
	}()
	defer u.Close()

	logger.Debug().Msg("Entering state lopp")
	for mgr.isRunning {
		//mgr.chanDoUI <- mgr.state
		select {
		case evt := <-mgr.chanOnBuilder:
			mgr.handleEventBuilder(logger, evt)
			mgr.chanDoUI <- mgr.state
		case evt := <-mgr.chanOnRunner:
			mgr.handleEventRunner(logger, evt)
			mgr.chanDoUI <- mgr.state
		case evt := <-mgr.chanOnUI:
			mgr.handleEventUI(logger, u, evt)
		}
		//mgr.chanDoWebserver <- mgr.state
	}
	logger.Debug().Msg("exiting state run loop")
	cancel()
	return nil
}
func (mgr *flogoStateManager) debugState(logger zerolog.Logger) {
	for k, p := range map[string]*state.Process{
		"builder.cur":  mgr.state.Builder.BuildCurrent,
		"builder.prev": mgr.state.Builder.BuildPrevious,
		"runner.cur":   mgr.state.Runner.RunCurrent,
		"runner.prev":  mgr.state.Runner.RunPrevious,
	} {
		if p == nil {
			logger.Info().Str("proc", k).Msg("nil")
			continue
		}
		status := "nil"
		if p.ExitCode != nil {
			status = fmt.Sprintf("%d", *p.ExitCode)
		}
		logger.Info().
			Str("proc", k).
			Str("status", status).
			Bytes("output", p.Output).
			Bytes("stderr", p.Stderr).
			Bytes("stdout", p.Stdout).
			Send()

	}

}
func (mgr *flogoStateManager) handleEventBuilder(logger zerolog.Logger, evt EventBuilder) {
	switch evt.Type {
	case EventBuildOutput:
		//logger.Debug().Msg("build output")
		mgr.state.Builder.BuildCurrent = evt.Process
	case EventBuildFailure:
		logger.Debug().Msg("build failure")
		mgr.state.Builder.Status = state.StatusBuilderFailed
		mgr.state.Builder.BuildCurrent = evt.Process
	case EventBuildStart:
		logger.Debug().Msg("build start")
		mgr.state.Builder.Status = state.StatusBuilderCompiling
		mgr.state.Builder.BuildPrevious = mgr.state.Builder.BuildCurrent
	case EventBuildSuccess:
		logger.Debug().Msg("build success")
		mgr.state.Builder.Status = state.StatusBuilderOK
		mgr.state.Builder.BuildCurrent = evt.Process
		mgr.chanDoRunner <- struct{}{}
	default:
		logger.Debug().Msg("build unknown")
	}
}
func (mgr *flogoStateManager) handleEventRunner(logger zerolog.Logger, evt EventRunner) {
	switch evt.Type {
	case EventRunnerOutput:
		mgr.state.Runner.RunCurrent = evt.Process
		logger.Debug().Msg("runner output")
		p := evt.Process
		logger.Info().
			Bytes("output", p.Output).
			Bytes("stderr", p.Stderr).
			Bytes("stdout", p.Stdout).
			Send()
		mgr.debugState(logger)
	case EventRunnerStart:
		logger.Debug().Msg("runner start")
		mgr.state.Runner.Status = state.StatusRunnerRunning
		mgr.state.Runner.RunCurrent = evt.Process
	case EventRunnerStopOK:
		logger.Debug().Msg("runner stop ok")
		mgr.state.Runner.Status = state.StatusRunnerStopOK
	case EventRunnerStopErr:
		logger.Debug().Msg("runner stop err")
		mgr.state.Runner.Status = state.StatusRunnerStopErr
	case EventRunnerWaiting:
		logger.Debug().Msg("runner waiting")
		mgr.state.Runner.Status = state.StatusRunnerStopErr
	default:
		logger.Debug().Msg("runner unknown")
	}
}
func (mgr *flogoStateManager) handleEventUI(logger zerolog.Logger, u ui.UI, evt ui.Event) {
	switch evt.Type {
	case ui.EventExit:
		mgr.isRunning = false
	}
}
