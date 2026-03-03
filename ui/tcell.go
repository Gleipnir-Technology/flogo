package ui

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	//"time"

	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/leaanthony/go-ansi-parser"
	"github.com/rs/zerolog/log"
)

type uiTcell struct {
	currentState *state.Flogo
	onEvent      chan Event
	screen       tcell.Screen
	target       string
	upstream     url.URL
}

func newUITcell(target string, upstream url.URL) (*uiTcell, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return nil, fmt.Errorf("failed to init screen: %w", err)
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(color.Reset).Foreground(color.Reset)
	screen.SetStyle(defStyle)

	screen.Clear()
	return &uiTcell{
		onEvent:  make(chan Event),
		screen:   screen,
		target:   target,
		upstream: upstream,
	}, nil
}
func (u *uiTcell) Close() {
	u.screen.Fini()
}
func (u *uiTcell) Events() <-chan Event {
	return u.onEvent
}
func (u *uiTcell) Run(ctx context.Context, chanOnEvent chan<- Event, chanNewState <-chan *state.Flogo) error {
	logger := log.Ctx(ctx).With().Caller().Logger()
	logger.Info().Msg("Started ui loop")
	u.drawInitial()
	for {
		u.screen.Show()
		select {
		case <-ctx.Done():
			logger.Debug().Msg("context ended, exiting UI")
			return nil
		case evt := <-u.screen.EventQ():
			logger.Debug().Msg("tcell event")
			e := convertEvent(evt)
			if e.Type != EventNone {
				chanOnEvent <- e
			}
			if e.Type == EventResize {
				u.redraw()
			}
		case s := <-chanNewState:
			logger.Debug().Msg("new ui state")
			u.currentState = s
			u.redraw()
		}
	}
}
func (u *uiTcell) drawBytesMultiline(start_x, start_y int, buffer []byte) {
	// If the bytes were ansi, convert them first
	parsed, err := ansi.Parse(string(buffer))
	if err != nil {
		log.Error().Err(err).Msg("failed to parse ANSI")
		return
	}
	// Convert from ANSI to tcell styles
	converted, err := ansiToTcell(parsed)
	if err != nil {
		log.Error().Err(err).Msg("failed to convert ANSI")
		return
	}
	u.drawStyledMultilineBottom(start_x, start_y, converted)
}
func (u *uiTcell) drawStyledMultilineBottom(start_x, start_y int, text []*styledText) {
	max_x, max_y := u.screen.Size()
	sections, err := fitToScreen(start_x, start_y, max_x, max_y, text)
	if err != nil {
		log.Error().Err(err).Msg("failed to fit to screen")
		return
	}
	u.drawStyledTextBottom(start_x, start_y, sections)
}

// drawStyledText renders styled segments to the tcell screen
func (u *uiTcell) drawStyledTextBottom(x_start, y_start int, lines [][]*styledText) {
	x := x_start
	_, y_max := u.screen.Size()
	// We draw the lines in reverse order so we ensure we are seeing the latest output
	y_count := y_max - y_start
	lines_len := len(lines)
	for y_offset := range y_count {
		idx_base := max(lines_len-y_count, 0)
		idx := idx_base + y_offset
		if idx >= lines_len {
			return
		}
		//log.Debug().Int("idx", idx).Int("lines_len", lines_len).Int("y_offset", y_offset).Send()
		line := lines[idx]
		y := y_start + y_offset
		for _, seg := range line {
			/*log.Debug().
			Str("fg", style.GetForeground().String()).
			Str("bg", style.GetBackground().String()).
			Send()*/
			for _, r := range seg.text {
				u.screen.SetContent(x, y, r, nil, seg.style)
				x++
			}
		}
		x = x_start
	}
}

func (u *uiTcell) drawInitial() {
	u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Starting up...")
}
func (u *uiTcell) redraw() {
	if u.currentState == nil {
		return
	}
	u.screen.Clear()

	// Draw title
	u.drawTitle(u.currentState)
	if u.currentState.Builder.Status != state.StatusBuilderOK {
		u.drawBuildStatus(u.currentState.Builder)
	} else {
		u.drawRunning(u.currentState.Runner)
	}

	u.screen.Show()
	u.screen.Sync()
}
func (u *uiTcell) drawBuildStatus(s *state.Builder) {
	if s == nil {
		return
	}
	style := tcell.StyleDefault.Foreground(color.White)
	var content string
	switch s.Status {
	case state.StatusBuilderFailed:
		style = tcell.StyleDefault.Foreground(color.Red)
		if s.BuildCurrent != nil {
			content = string(s.BuildCurrent.Output)
		} else if s.BuildPrevious != nil {
			content = string(s.BuildPrevious.Output)
		} else {
			content = "flogo: no build output to show."
		}
	case state.StatusBuilderCompiling:
		style = tcell.StyleDefault.Foreground(color.Yellow)
		if s.BuildCurrent == nil {
			content = "flogo: no output yet"
		} else if len(s.BuildCurrent.Output) > 0 {
			content = string(s.BuildCurrent.Output)
		} else {
			content = "flogo: compiling..."
		}
	default:
		style = tcell.StyleDefault.Foreground(color.Purple)
		content = "flogo: programmer error (build)"
	}
	u.drawBuildOutput(0, 1, content, style)
}

// Given some build output, add styling and fit it to screen
func (u *uiTcell) drawBuildOutput(start_x, start_y int, content string, style tcell.Style) {
	parsed, err := parseGoBuildOutput(content)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse go build output")
		return
	}
	styled := make([]*styledText, 0)
	filename_style := tcell.StyleDefault.Foreground(color.Blue)
	error_style := tcell.StyleDefault.Foreground(color.Red)
	for _, p := range parsed {
		styled = append(styled, &styledText{
			style: filename_style,
			text:  fmt.Sprintf("%s:%d:%d ", p.Filename, p.Line, p.Column),
		})
		styled = append(styled, &styledText{
			style: error_style,
			text:  p.Message + "\n",
		})
	}

	max_x, max_y := u.screen.Size()
	sections, err := fitToScreen(start_x, start_y, max_x, max_y, styled)
	if err != nil {
		log.Error().Err(err).Msg("failed to fit to screen")
		return
	}
	u.drawStyledTextBottom(start_x, start_y, sections)
}
func (u *uiTcell) drawCompilation(state *state.Flogo) {
	u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), "Compiling...")
}
func (u *uiTcell) drawRunning(s *state.Runner) {
	if s == nil {
		return
	}

	switch s.Status {
	case state.StatusRunnerRunning, state.StatusRunnerStopErr, state.StatusRunnerStopOK:
		if s.RunCurrent == nil {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: no runCurrent.")
		} else if len(s.RunCurrent.Output) > 0 {
			u.drawBytesMultiline(0, 1, s.RunCurrent.Output)
		} else if s.RunPrevious == nil {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: no runPrevious.")
		} else {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: maybe use previous output...?")
		}
	case state.StatusRunnerWaiting:
		u.drawText(0, 1, tcell.StyleDefault, "flogo: waiting...")
	default:
		u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Purple), "flogo: programmer error (run)")
	}
}
func (u *uiTcell) drawStatus(status string, style tcell.Style) {
	u.drawText(0, 1, style.Bold(true), fmt.Sprintf("Status: %s", status))
}
func (u *uiTcell) drawText(x, y int, style tcell.Style, text string) {
	for i, r := range text {
		u.screen.SetContent(x+i, y, r, nil, style)
	}
}
func (u *uiTcell) drawTitle(s *state.Flogo) {
	switch s.Builder.Status {
	case state.StatusBuilderCompiling:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Compiling")
	case state.StatusBuilderFailed:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Red).Bold(true), "Failed")
	case state.StatusBuilderOK:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Idle")
	default:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Purple).Bold(true), "Unknown")
	}

	switch s.Runner.Status {
	case state.StatusRunnerRunning:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Running")
	case state.StatusRunnerStopErr:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Red).Bold(true), "Failed")
	case state.StatusRunnerStopOK:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Exited")
	case state.StatusRunnerWaiting:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Blue).Bold(true), "Waiting...")
	default:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Purple).Bold(true), "Unknown")
	}
	u.drawText(20, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), u.upstream.String())
}
func (u *uiTcell) sync() {
	u.screen.Sync()
}

func convertEvent(evt tcell.Event) Event {
	logger := log.Logger
	logger.Debug().Msg("ui event")
	switch ev := evt.(type) {
	case *tcell.EventClipboard:
		logger.Info().Msg("event clipboard")
		return Event{Type: EventNone}
	case *tcell.EventError:
		logger.Info().Msg("event error")
		return Event{Type: EventNone}
	case *tcell.EventFocus:
		logger.Info().Msg("event focus")
		return Event{Type: EventNone}
	case *tcell.EventInterrupt:
		logger.Info().Msg("event interrupt")
		return Event{Type: EventNone}
	case *tcell.EventKey:
		logger.Info().Msg("event key")
		if ev.Key() == tcell.KeyCtrlC {
			logger.Debug().Msg("SIGINT, exiting")
			return Event{Type: EventExit}
		} else if ev.Str() == " " {
			return Event{Type: EventUpdate}
		} else if ev.Str() == "d" {
			return Event{Type: EventDebug}
		} else if ev.Str() == "r" {
			return Event{Type: EventRestart}
		} else {
			logger.Debug().Msg("updating webserver from keypress")
			return Event{Type: EventUpdate}
		}
	case *tcell.EventMouse:
		logger.Info().Msg("event mouse")
		return Event{Type: EventNone}
	case *tcell.EventPaste:
		logger.Info().Msg("event paste")
		return Event{Type: EventNone}
	case *tcell.EventResize:
		return Event{Type: EventResize}
	case *tcell.EventTime:
		logger.Info().Msg("event time")
		return Event{Type: EventNone}
	default:
		t := reflect.TypeOf(evt)
		if t == nil {
			logger.Info().Msg("unrecognized nil event")
		} else {
			logger.Info().Str("type", t.Name()).Msg("unrecognized event")
		}
		return Event{Type: EventNone}
	}
}
