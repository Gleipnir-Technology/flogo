package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
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
		upstream = "http://localhost:8080" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		fmt.Printf("Failed to parse '%s' as a URL: %v\n", upstream, err)
		os.Exit(2)
	}

	// Create a context that we can cancel for signaling all goroutines to clean up
	ctx, cancel := context.WithCancel(log.With().Str("component", "arcgis").Logger().WithContext(context.Background()))
	defer cancel()

	// Create a channel where all the goroutines can signal death

	chan_to_build := make(chan struct{})
	chan_builder_events := make(chan EventBuilder)
	something_died := make(chan error)

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
		ToBuild:  chan_to_build,
	}
	go builder.Run(ctx)

	// Start the UI in a goroutine

	u, err := newUI()
	if err != nil {
		fmt.Printf("Failed to create UI: %+v\n", err)
		os.Exit(3)
	}
	defer u.Fini()

	// Start the web server
	go startServer(bind)

	// Handle keyboard input
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	log.Info().Msg("entering main loop")

	event_q := u.EventQ()
	is_running := true
	for is_running {
		u.Sync()
		select {
		case death := <-something_died:
			fmt.Printf("Death: %v\n", death)
			cancel()
		case evt := <-chan_builder_events:
			switch evt.Type {
			case EventBuildFailure:
				u.state.isCompiling = false
				u.state.lastBuildOutput = evt.Message
				u.state.lastBuildSuccess = false
			case EventBuildStart:
				u.state.isCompiling = true
			case EventBuildSuccess:
				u.state.isCompiling = false
				u.state.lastBuildOutput = evt.Message
				u.state.lastBuildSuccess = true
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
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				is_running = false
			case syscall.SIGHUP:
				reload()
			}
		}
	}
	cancel()
	u.Fini()
}
func reload() {
}
func setupLogging(file *os.File) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	writer := zerolog.NewConsoleWriter()
	writer.Out = file
	writer.FormatLevel = func(i interface{}) string { return strings.ToUpper(fmt.Sprintf("%-6s", i)) }
	writer.FormatFieldName = func(i interface{}) string { return fmt.Sprintf("%s:", i) }
	writer.FormatPartValueByName = func(i interface{}, s string) string {
		var ret string
		switch s {
		case "one":
			ret = strings.ToUpper(fmt.Sprintf("%s", i))
		case "two":
			ret = strings.ToLower(fmt.Sprintf("%s", i))
		case "three":
			ret = strings.ToLower(fmt.Sprintf("(%s)", i))
		}
		return ret
	}
	log.Logger = zerolog.New(writer)
}
