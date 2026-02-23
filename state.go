package main

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/rs/zerolog/log"
)

type stateFlogo struct {
	builder *stateBuilder
	runner  *stateRunner
}
type stateProcess struct {
	exitCode *int
	output   []byte
	stderr   []byte
	stdout   []byte
}
type stateBuilder struct {
	buildPrevious *stateProcess
	buildCurrent  *stateProcess
	status        statusBuilder
}
type stateRunner struct {
	runPrevious *stateProcess
	runCurrent  *stateProcess
	status      statusRunner
}
type flogoStateManager struct {
	chanToBuild         chan struct{}
	chanBuilderEvents   chan EventBuilder
	chanRunnerEvents    chan EventRunner
	chanRunnerRestart   chan struct{}
	chanSomethingDied   chan error
	chanWebserverChange chan *stateFlogo
	isRunning           bool
	state               stateFlogo
}
type statusBuilder int

const (
	statusBuilderCompiling statusBuilder = iota
	statusBuilderFailed
	statusBuilderOK
)

func StatusStringBuilder(s statusBuilder) string {
	switch s {
	case statusBuilderCompiling:
		return "compiling"
	case statusBuilderFailed:
		return "failed"
	case statusBuilderOK:
		return "ok"
	}
	return "unknown"
}

type statusRunner int

const (
	statusRunnerRunning statusRunner = iota
	statusRunnerStopOK
	statusRunnerStopErr
	statusRunnerWaiting
)

func StatusStringRunner(s statusRunner) string {
	switch s {
	case statusRunnerRunning:
		return "running"
	case statusRunnerStopOK:
		return "ok"
	case statusRunnerStopErr:
		return "error"
	case statusRunnerWaiting:
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
		chanWebserverChange: make(chan *stateFlogo, 10),
		isRunning:           true,
		state: stateFlogo{
			builder: &stateBuilder{
				buildPrevious: nil,
				buildCurrent:  nil,
				status:        statusBuilderOK,
			},
			runner: &stateRunner{
				runPrevious: nil,
				runCurrent:  nil,
				status:      statusRunnerWaiting,
			},
		},
	}
}

func (mgr *flogoStateManager) Run(bind string, target string, enable_tui bool) error {
	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Logger().WithContext(context.Background()))
	defer cancel()

	// Create channels for goroutine comms

	watcher := Watcher{
		OnBuild: mgr.chanToBuild,
		Target:  target,
	}
	go func() {
		err := watcher.Run(ctx)
		if err != nil {
			mgr.chanSomethingDied <- fmt.Errorf("watcher died: %w", err)
		}
	}()

	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  mgr.chanBuilderEvents,
		Target:   target,
		ToBuild:  mgr.chanToBuild,
	}
	go func() {
		err := builder.Run(ctx)
		if err != nil {
			mgr.chanSomethingDied <- fmt.Errorf("builder died: %w", err)
		}
	}()
	mgr.chanToBuild <- struct{}{}

	runner := Runner{
		DoRestart: mgr.chanRunnerRestart,
		OnDeath:   mgr.chanSomethingDied,
		OnEvent:   mgr.chanRunnerEvents,
		Target:    target,
	}
	go func() {
		err := runner.Run(ctx)
		if err != nil {
			mgr.chanSomethingDied <- fmt.Errorf("runner died: %w", err)
		}
	}()

	// Start the UI in a goroutine

	var u *ui
	if enable_tui {
		var err error
		u, err = newUI(target, *upstreamURL)
		if err != nil {
			return fmt.Errorf("Failed to create UI: %w\n", err)
		}
		defer u.Fini()
	}

	// Start the web server
	ws := NewWebserver(mgr.chanWebserverChange)
	go ws.Start(ctx, bind, *upstreamURL)

	var chan_event_ui chan tcell.Event
	if u != nil {
		chan_event_ui = u.EventQ()
	}
	var cause_of_death error
	for mgr.isRunning {
		if u != nil {
			u.Redraw(&mgr.state)
		}
		select {
		case cause_of_death = <-mgr.chanSomethingDied:
			log.Error().Err(cause_of_death).Msg("something died")
			mgr.isRunning = false
		case evt := <-mgr.chanBuilderEvents:
			mgr.handleEventBuilder(evt)
		case evt := <-mgr.chanRunnerEvents:
			mgr.handleEventRunner(evt)
		case evt := <-chan_event_ui:
			mgr.handleEventUI(u, evt)
		}
		mgr.updateWebserver()
	}
	log.Debug().Msg("exiting state run loop")
	cancel()
	u.Fini()
	if cause_of_death != nil {
		return fmt.Errorf("Instadeath: %w", cause_of_death)
	}
	return nil
}
func (mgr *flogoStateManager) handleEventBuilder(evt EventBuilder) {
	switch evt.Type {
	case EventBuildOutput:
		log.Debug().Msg("build output")
		mgr.state.builder.buildCurrent = evt.Process
	case EventBuildFailure:
		log.Debug().Msg("build failure")
		mgr.state.builder.status = statusBuilderFailed
		mgr.state.builder.buildCurrent = evt.Process
	case EventBuildStart:
		log.Debug().Msg("build start")
		mgr.state.builder.status = statusBuilderCompiling
		mgr.state.builder.buildPrevious = mgr.state.builder.buildCurrent
	case EventBuildSuccess:
		log.Debug().Msg("build success")
		mgr.state.builder.status = statusBuilderOK
		mgr.state.builder.buildCurrent = evt.Process
		mgr.chanRunnerRestart <- struct{}{}
	default:
		log.Debug().Msg("build unknown")
	}
}
func (mgr *flogoStateManager) handleEventRunner(evt EventRunner) {
	switch evt.Type {
	case EventRunnerOutput:
		log.Debug().Msg("runner output")
		mgr.state.runner.runCurrent = evt.Process
	case EventRunnerStart:
		log.Debug().Msg("runner start")
		mgr.state.runner.status = statusRunnerRunning
		mgr.state.runner.runCurrent = evt.Process
	case EventRunnerStopOK:
		log.Debug().Msg("runner stop ok")
		mgr.state.runner.status = statusRunnerStopOK
	case EventRunnerStopErr:
		log.Debug().Msg("runner stop err")
		mgr.state.runner.status = statusRunnerStopErr
	case EventRunnerWaiting:
		log.Debug().Msg("runner waiting")
		mgr.state.runner.status = statusRunnerStopErr
	default:
		log.Debug().Msg("runner unknown")
	}
}
func (mgr *flogoStateManager) handleEventUI(u *ui, evt tcell.Event) {
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
		log.Info().Msg("event key")
		if ev.Key() == tcell.KeyCtrlC {
			log.Debug().Msg("SIGINT, exiting")
			mgr.isRunning = false
		} else {
			log.Debug().Msg("updating webserver from keypress")
			mgr.updateWebserver()
		}
	case *tcell.EventMouse:
		log.Info().Msg("event mouse")
	case *tcell.EventPaste:
		log.Info().Msg("event paste")
	case *tcell.EventResize:
		log.Info().Msg("event resize")
		u.Redraw(&mgr.state)
		u.Sync()
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
