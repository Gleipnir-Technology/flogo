package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	//"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/rs/zerolog/log"
)

type ui struct {
	screen   tcell.Screen
	target   string
	upstream url.URL
}

func newUI(target string, upstream url.URL) (*ui, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return nil, fmt.Errorf("failed to init screen: %w", err)
	}
	screen.Clear()
	return &ui{
		screen:   screen,
		target:   target,
		upstream: upstream,
	}, nil
}
func (u ui) EventQ() chan tcell.Event {
	return u.screen.EventQ()
}
func (u ui) Fini() {
	u.screen.Fini()
}
func (u ui) Run(ctx context.Context) {
	//var err error
	if err := u.screen.Init(); err != nil {
		log.Error().Err(err).Msg("UI run failure")
		return
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(color.Reset).Foreground(color.Reset)
	u.screen.SetStyle(defStyle)

	// Clear screen
	u.screen.Clear()
}
func (u ui) Redraw(state *stateFlogo) {
	if state == nil {
		return
	}
	u.screen.Clear()

	// Draw title
	u.drawTitle(state)
	// Draw upstream info
	//u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), fmt.Sprintf("Upstream: %s", upstreamURL.String()))

	if state.builder.status != statusBuilderOK {
		u.drawBuildStatus(state.builder)
	} else {
		u.drawRunning(state.runner)
	}

	u.screen.Show()
}
func (u ui) Sync() {
	u.screen.Sync()
}

func (u ui) drawBuildStatus(state *stateBuilder) {
	if state == nil {
		return
	}
	style := tcell.StyleDefault.Foreground(color.White)
	var content string
	switch state.status {
	case statusBuilderFailed:
		style = tcell.StyleDefault.Foreground(color.Red)
		if len(state.buildCurrent.stderr) > 0 {
			content = string(state.buildCurrent.stderr)
		} else if len(state.buildCurrent.stdout) > 0 {
			content = string(state.buildCurrent.stdout)
		} else if state.buildPrevious != nil {
			if len(state.buildPrevious.stderr) > 0 {
				content = string(state.buildPrevious.stderr)
			} else if len(state.buildPrevious.stdout) > 0 {
				content = string(state.buildPrevious.stdout)
			} else {
				content = "flogo: no output to show."
			}
		} else {
			content = "flogo: no output to show."
		}
	case statusBuilderCompiling:
		style = tcell.StyleDefault.Foreground(color.Yellow)
		if state.buildCurrent == nil {
			content = "flogo: no output yet"
		} else if len(state.buildCurrent.stderr) > 0 {
			content = string(state.buildCurrent.stderr)
		} else if len(state.buildCurrent.stdout) > 0 {
			content = string(state.buildCurrent.stdout)
		} else {
			content = "flogo: waiting..."
		}
	default:
		style = tcell.StyleDefault.Foreground(color.Purple)
		content = "flogo: programmer error (build)"
	}
	u.drawContent(content, style)
}

func (u ui) drawContent(content string, style tcell.Style) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i < 15 { // Limit number of lines to avoid overflow
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(color.White), line)
		} else if i == 15 {
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(color.White), "... (more errors)")
			break
		}
	}
}
func (u ui) drawCompilation(state *stateFlogo) {
	u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), "Compiling...")
}
func (u ui) drawRunning(state *stateRunner) {
	if state == nil {
		return
	}

	switch state.status {
	case statusRunnerRunning:
		if state.runCurrent == nil {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: no content yet")
		} else if len(state.runCurrent.stderr) > 0 {
			u.drawBytesMultiline(0, 1, tcell.StyleDefault, state.runCurrent.stderr)
		} else if len(state.runCurrent.stdout) > 0 {
			u.drawBytesMultiline(0, 1, tcell.StyleDefault, state.runCurrent.stdout)
		} else if state.runPrevious != nil {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: maybe use previous output...?")
		} else {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: no output to show.")
		}
	case statusRunnerStopOK:
		u.drawText(0, 1, tcell.StyleDefault, "flogo: stopped.")
	case statusRunnerStopErr:
		if len(state.runCurrent.stderr) > 0 {
			u.drawBytesMultiline(0, 1, tcell.StyleDefault, state.runCurrent.stderr)
		} else if len(state.runCurrent.stdout) > 0 {
			u.drawBytesMultiline(0, 1, tcell.StyleDefault, state.runCurrent.stdout)
		} else if state.runPrevious != nil {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: maybe use previous output...?")
		} else {
			u.drawText(0, 1, tcell.StyleDefault, "flogo: no output to show.")
		}
	case statusRunnerWaiting:
		u.drawText(0, 1, tcell.StyleDefault, "flogo: waiting...")
	default:
		u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Purple), "flogo: programmer error (run)")
	}
}
func (u ui) drawStatus(status string, style tcell.Style) {
	u.drawText(0, 1, style.Bold(true), fmt.Sprintf("Status: %s", status))
}
func (u ui) drawText(x, y int, style tcell.Style, text string) {
	for i, r := range text {
		u.screen.SetContent(x+i, y, r, nil, style)
	}
}
func (u ui) drawBytesMultiline(x, y int, style tcell.Style, buffer []byte) {
	// Convert the buffer into ansi sequences
	text, err := ParseANSI(buffer)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse ANSI")
		return
	}
	DrawStyledText(u.screen, x, y, text)
}
func (u ui) drawTitle(state *stateFlogo) {
	switch state.builder.status {
	case statusBuilderCompiling:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Compiling")
	case statusBuilderFailed:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Red).Bold(true), "Failed")
	case statusBuilderOK:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Idle")
	default:
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Purple).Bold(true), "Unknown")
	}

	switch state.runner.status {
	case statusRunnerRunning:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Running")
	case statusRunnerStopErr:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Red).Bold(true), "Failed")
	case statusRunnerStopOK:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Exited")
	case statusRunnerWaiting:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Blue).Bold(true), "Waiting...")
	default:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Purple).Bold(true), "Unknown")
	}
	u.drawText(20, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), u.upstream.String())
}
