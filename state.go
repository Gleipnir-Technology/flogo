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
	builderStatus    builderStatus
	lastBuildOutput  string
	lastBuildSuccess bool
	lastRunStdout    []byte
	lastRunStderr    []byte
	runnerStatus     runnerStatus
}
type flogoStateManager struct {
	chanToBuild         chan struct{}
	chanBuilderEvents   chan EventBuilder
	chanRunnerEvents    chan EventRunner
	chanRunnerRestart   chan struct{}
	chanSomethingDied   chan error
	chanWebserverChange chan *flogoState
	isRunning           bool
	state               flogoState
}
type builderStatus int

const (
	builderStatusCompiling builderStatus = iota
	builderStatusFailed
	builderStatusOK
)

func StatusStringBuilder(s builderStatus) string {
	switch s {
	case builderStatusCompiling:
		return "compiling"
	case builderStatusFailed:
		return "failed"
	case builderStatusOK:
		return "ok"
	}
	return "unknown"
}

type runnerStatus int

const (
	runnerStatusRunning runnerStatus = iota
	runnerStatusStopOK
	runnerStatusStopErr
	runnerStatusWaiting
)

func StatusStringRunner(s runnerStatus) string {
	switch s {
	case runnerStatusRunning:
		return "running"
	case runnerStatusStopOK:
		return "ok"
	case runnerStatusStopErr:
		return "error"
	case runnerStatusWaiting:
		return "waiting"
	}
	return "unknown"
}

func newFlogoStateManager() flogoStateManager {
	return flogoStateManager{
		chanToBuild:         make(chan struct{}),
		chanBuilderEvents:   make(chan EventBuilder),
		chanRunnerEvents:    make(chan EventRunner),
		chanRunnerRestart:   make(chan struct{}),
		chanSomethingDied:   make(chan error),
		chanWebserverChange: make(chan *flogoState, 10),
		isRunning:           true,
		state: flogoState{
			builderStatus:    builderStatusOK,
			lastBuildOutput:  "",
			lastBuildSuccess: false,
			lastRunStdout:    []byte{},
			lastRunStderr:    []byte{},
			runnerStatus:     runnerStatusWaiting,
		},
	}
}

func (mgr *flogoStateManager) Run(bind string, target string) error {
	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Logger().WithContext(context.Background()))
	defer cancel()

	// Create channels for goroutine comms

	watcher := Watcher{
		OnBuild: mgr.chanToBuild,
		OnDeath: mgr.chanSomethingDied,
		Target:  target,
	}
	go watcher.Run(ctx)

	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  mgr.chanBuilderEvents,
		OnDeath:  mgr.chanSomethingDied,
		Target:   target,
		ToBuild:  mgr.chanToBuild,
	}
	go builder.Run(ctx)
	mgr.chanToBuild <- struct{}{}

	runner := Runner{
		DoRestart: mgr.chanRunnerRestart,
		OnDeath:   mgr.chanSomethingDied,
		OnEvent:   mgr.chanRunnerEvents,
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
	ws := NewWebserver(mgr.chanWebserverChange)
	go ws.Start(ctx, bind, *upstreamURL)

	event_q := u.EventQ()
	var cause_of_death error
	for mgr.isRunning {
		u.Sync(&mgr.state)
		select {
		case cause_of_death := <-mgr.chanSomethingDied:
			log.Error().Err(cause_of_death).Msg("something died")
			mgr.isRunning = false
		case evt := <-mgr.chanBuilderEvents:
			mgr.handleEventBuilder(evt)
		case evt := <-mgr.chanRunnerEvents:
			mgr.handleEventRunner(evt)
		case evt := <-event_q:
			mgr.handleEventUI(evt)
		}
		mgr.updateWebserver()
	}
	cancel()
	u.Fini()
	if cause_of_death != nil {
		return fmt.Errorf("Instadeath: %w", cause_of_death)
	}
	return nil
}
func (mgr *flogoStateManager) handleEventBuilder(evt EventBuilder) {
	switch evt.Type {
	case EventBuildFailure:
		mgr.state.builderStatus = builderStatusFailed
		mgr.state.lastBuildOutput = evt.Message
		mgr.state.lastBuildSuccess = false
	case EventBuildStart:
		mgr.state.builderStatus = builderStatusCompiling
	case EventBuildSuccess:
		mgr.state.builderStatus = builderStatusOK
		mgr.state.lastBuildOutput = evt.Message
		mgr.state.lastBuildSuccess = true
		mgr.chanRunnerRestart <- struct{}{}
	default:
		log.Debug().Msg("build unknown")
	}
}
func (mgr *flogoStateManager) handleEventRunner(evt EventRunner) {
	switch evt.Type {
	case EventRunnerStart:
		mgr.state.runnerStatus = runnerStatusRunning
		mgr.state.lastRunStdout = []byte{}
		mgr.state.lastRunStderr = []byte{}
	case EventRunnerStopOK:
		mgr.state.runnerStatus = runnerStatusStopOK
	case EventRunnerStopErr:
		mgr.state.runnerStatus = runnerStatusStopErr
	case EventRunnerStdout:
		mgr.state.lastRunStdout = evt.Buffer
	case EventRunnerStderr:
		mgr.state.lastRunStderr = evt.Buffer
	case EventRunnerWaiting:
		mgr.state.runnerStatus = runnerStatusStopErr
	default:
		log.Debug().Msg("runner unknown")
	}
}
func (mgr *flogoStateManager) handleEventUI(evt tcell.Event) {
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
			mgr.isRunning = false
		} else {
			mgr.updateWebserver()
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
func (mgr *flogoStateManager) updateWebserver() {
	mgr.chanWebserverChange <- &mgr.state
}
