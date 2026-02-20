package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	//"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	lastBuildOutput  string
	lastBuildSuccess bool
	uiMutex          sync.Mutex
)

func main() {
	var err error
	var target = flag.String("target", ".", "The directory containing the go project to build")
	flag.Parse()

	file, err := os.OpenFile(
		"flogo.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		fmt.Println("Failed to open 'flogo.log' for writing.")
		os.Exit(1)
	}
	defer file.Close()
	setupLogging(file)

	bind := os.Getenv("FLOGO_BIND")
	if bind == "" {
		bind = ":10000" // Default if not specified
	}
	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:9001" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		fmt.Printf("Failed to parse '%s' as a URL: %v\n", upstream, err)
		os.Exit(2)
	}

	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Logger().WithContext(context.Background()))
	defer cancel()

	// Create channels for goroutine comms
	chan_to_build := make(chan struct{})
	chan_builder_events := make(chan EventBuilder)
	chan_runner_restart := make(chan struct{})
	chan_runner_events := make(chan EventRunner)
	something_died := make(chan error)

	// Keep track of the state of everything
	state := newFlogoState()

	watcher := Watcher{
		OnBuild: chan_to_build,
		OnDeath: something_died,
		Target:  *target,
	}
	go watcher.Run(ctx)

	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  chan_builder_events,
		OnDeath:  something_died,
		Target:   *target,
		ToBuild:  chan_to_build,
	}
	go builder.Run(ctx)
	chan_to_build <- struct{}{}

	runner := Runner{
		DoRestart: chan_runner_restart,
		OnDeath:   something_died,
		OnEvent:   chan_runner_events,
		Target:    *target,
	}
	go runner.Run(ctx)

	// Start the UI in a goroutine

	u, err := newUI(*target, *upstreamURL)
	if err != nil {
		fmt.Printf("Failed to create UI: %+v\n", err)
		os.Exit(3)
	}
	defer u.Fini()

	// Start the web server
	go startServer(ctx, bind, *upstreamURL)

	// Handle keyboard input
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	log.Info().Msg("entering main loop")

	event_q := u.EventQ()
	is_running := true
	var cause_of_death error
	for is_running {
		u.Sync(&state)
		select {
		case cause_of_death := <-something_died:
			log.Error().Err(cause_of_death).Msg("something died")
			is_running = false
		case evt := <-chan_builder_events:
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
				chan_runner_restart <- struct{}{}
			default:
				log.Debug().Msg("build unknown")
			}
		case evt := <-chan_runner_events:
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
		case evt := <-event_q:
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
					is_running = false
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
		case sig := <-c:
			log.Debug().Msg("signal")
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				log.Debug().Msg("sigint/sigterm")
				is_running = false
			case syscall.SIGHUP:
				log.Debug().Msg("sighup")
				reload()
			}
		}
	}
	cancel()
	u.Fini()
	if cause_of_death != nil {
		fmt.Printf("Instadeath: %+v\n", cause_of_death)
	}
}
func reload() {
	log.Info().Msg("fake reload")
}
func setupLogging(file *os.File) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// Track start time for delta timestamps
	startTime := time.Now()

	writer := zerolog.ConsoleWriter{
		Out:        file,
		TimeFormat: "15:04:05", // placeholder, will be overridden
		NoColor:    false,      // Enable colors for tail -f
	}

	// Custom timestamp formatter showing elapsed time
	writer.FormatTimestamp = func(i interface{}) string {
		elapsed := time.Since(startTime)
		return fmt.Sprintf("\x1b[90m[+%s]\x1b[0m",
			elapsed.Round(time.Millisecond))
	}

	// Create logger with timestamp
	log.Logger = zerolog.New(writer).With().Timestamp().Logger()
}
