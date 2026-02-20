package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/rs/zerolog/log"
)

type flogoState struct {
	builderStatus     builderStatus
	chanToBuild       chan struct{}
	chanBuilderEvents chan EventBuilder
	chanRunnerEvents  chan EventRunner
	chanRunnerRestart chan struct{}
	chanSomethingDied chan error
	isRunning         bool
	lastBuildOutput   string
	lastBuildSuccess  bool
	lastRunStdout     []byte
	lastRunStderr     []byte
	runnerStatus      runnerStatus
}
type builderStatus int

const (
	builderStatusCompiling builderStatus = iota
	builderStatusFailed
	builderStatusOK
)

type runnerStatus int

const (
	runnerStatusRunning runnerStatus = iota
	runnerStatusStopOK
	runnerStatusStopErr
	runnerStatusWaiting
)

func newFlogoState() flogoState {
	return flogoState{
		builderStatus:     builderStatusOK,
		chanToBuild:       make(chan struct{}),
		chanBuilderEvents: make(chan EventBuilder),
		chanRunnerEvents:  make(chan EventRunner),
		chanRunnerRestart: make(chan struct{}),
		chanSomethingDied: make(chan error),
		isRunning:         true,
		lastBuildOutput:   "",
		lastBuildSuccess:  false,
		lastRunStdout:     []byte{},
		lastRunStderr:     []byte{},
		runnerStatus:      runnerStatusWaiting,
	}
}

func (state *flogoState) Run(bind string, target string) error {
	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Logger().WithContext(context.Background()))
	defer cancel()

	// Create channels for goroutine comms

	watcher := Watcher{
		OnBuild: state.chanToBuild,
		OnDeath: state.chanSomethingDied,
		Target:  target,
	}
	go watcher.Run(ctx)

	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  state.chanBuilderEvents,
		OnDeath:  state.chanSomethingDied,
		Target:   target,
		ToBuild:  state.chanToBuild,
	}
	go builder.Run(ctx)
	state.chanToBuild <- struct{}{}

	runner := Runner{
		DoRestart: state.chanRunnerRestart,
		OnDeath:   state.chanSomethingDied,
		OnEvent:   state.chanRunnerEvents,
		Target:    target,
	}
	go runner.Run(ctx)

	// Start the UI in a goroutine

	u, err := newUI(target, *upstreamURL)
	if err != nil {
		fmt.Printf("Failed to create UI: %+v\n", err)
		os.Exit(3)
	}
	defer u.Fini()

	// Start the web server
	go startServer(ctx, bind, *upstreamURL)

	event_q := u.EventQ()
	var cause_of_death error
	for state.isRunning {
		u.Sync(state)
		select {
		case cause_of_death := <-state.chanSomethingDied:
			log.Error().Err(cause_of_death).Msg("something died")
			state.isRunning = false
		case evt := <-state.chanBuilderEvents:
			state.handleEventBuilder(evt)
		case evt := <-state.chanRunnerEvents:
			state.handleEventRunner(evt)
		case evt := <-event_q:
			state.handleEventUI(evt)
		}
	}
	cancel()
	u.Fini()
	if cause_of_death != nil {
		return fmt.Errorf("Instadeath: %w", cause_of_death)
	}
	return nil
}
func (state *flogoState) handleEventBuilder(evt EventBuilder) {
	switch evt.Type {
	case EventBuildFailure:
		state.builderStatus = builderStatusFailed
		state.lastBuildOutput = evt.Message
		state.lastBuildSuccess = false
	case EventBuildStart:
		state.builderStatus = builderStatusCompiling
	case EventBuildSuccess:
		state.builderStatus = builderStatusOK
		state.lastBuildOutput = evt.Message
		state.lastBuildSuccess = true
		state.chanRunnerRestart <- struct{}{}
	default:
		log.Debug().Msg("build unknown")
	}
}
func (state *flogoState) handleEventRunner(evt EventRunner) {
	switch evt.Type {
	case EventRunnerStart:
		state.runnerStatus = runnerStatusRunning
		state.lastRunStdout = []byte{}
		state.lastRunStderr = []byte{}
	case EventRunnerStopOK:
		state.runnerStatus = runnerStatusStopOK
	case EventRunnerStopErr:
		state.runnerStatus = runnerStatusStopErr
	case EventRunnerStdout:
		state.lastRunStdout = evt.Buffer
	case EventRunnerStderr:
		state.lastRunStderr = evt.Buffer
	case EventRunnerWaiting:
		state.runnerStatus = runnerStatusStopErr
	default:
		log.Debug().Msg("runner unknown")
	}
}
func (state *flogoState) handleEventUI(evt tcell.Event) {
	switch ev := evt.(type) {
	case *tcell.EventClipboard:
		log.Info().Msg("event clipboard")
	case *tcell.EventError:
		log.Info().Msg("event error")
	case *tcell.EventFocus:
		log.Info().Msg("event focus")
	case *tcell.EventInterrupt:
		log.Info().Msg("event interrupt")
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyCtrlC {
			state.isRunning = false
		}
		log.Info().Msg("event key")
	case *tcell.EventMouse:
		log.Info().Msg("event mouse")
	case *tcell.EventPaste:
		log.Info().Msg("event paste")
	case *tcell.EventResize:
		log.Info().Msg("event resize")
	case *tcell.EventTime:
		log.Info().Msg("event time")
	default:
		t := reflect.TypeOf(evt)
		if t == nil {
			log.Info().Msg("unrecognized nil event")
		} else {
			log.Info().Str("type", t.Name()).Msg("unrecognized event")
		}
	}
}
