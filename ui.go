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
func (u ui) Redraw(state *flogoState) {
	if state == nil {
		return
	}
	u.screen.Clear()

	// Draw title
	u.drawTitle(state)
	// Draw upstream info
	//u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), fmt.Sprintf("Upstream: %s", upstreamURL.String()))

	if !state.lastBuildSuccess && state.lastBuildOutput != "" {
		u.drawBuildFailure(state)
	} else if state.builderStatus == builderStatusCompiling {
		u.drawCompilation(state)
	} else {
		u.drawRunning(state)
	}

	u.screen.Show()
}
func (u ui) Sync() {
	u.screen.Sync()
}

func (u ui) drawBuildFailure(state *flogoState) {
	u.drawStatus("Build Failed. Errors:", tcell.StyleDefault.Foreground(color.Red))

	// Split output into lines and display them
	lines := strings.Split(state.lastBuildOutput, "\n")
	for i, line := range lines {
		if i < 15 { // Limit number of lines to avoid overflow
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(color.White), line)
		} else if i == 15 {
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(color.White), "... (more errors)")
			break
		}
	}
}
func (u ui) drawCompilation(state *flogoState) {
	u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), "Compiling...")
}
func (u ui) drawRunning(state *flogoState) {
	if state == nil {
		return
	}
	if len(state.lastRunStderr) > 0 {
		u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Yellow), "stderr:")
		u.drawBytesMultiline(0, 2, tcell.StyleDefault.Foreground(color.White), state.lastRunStderr)
	} else if len(state.lastRunStdout) > 0 {
		u.drawText(0, 1, tcell.StyleDefault.Foreground(color.Green), "stdout:")
		u.drawBytesMultiline(0, 2, tcell.StyleDefault.Foreground(color.White), state.lastRunStdout)
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
func (u ui) drawTitle(state *flogoState) {
	if state.builderStatus == builderStatusCompiling {
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Compiling")
	} else {
		u.drawText(0, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Idle")
	}

	switch state.runnerStatus {
	case runnerStatusRunning:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Yellow).Bold(true), "Running")
	case runnerStatusStopErr:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Red).Bold(true), "Failed")
	case runnerStatusStopOK:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), "Exited")
	case runnerStatusWaiting:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Blue).Bold(true), "Waiting...")
	default:
		u.drawText(10, 0, tcell.StyleDefault.Foreground(color.Purple).Bold(true), "Unknown")
	}
	u.drawText(20, 0, tcell.StyleDefault.Foreground(color.Green).Bold(true), u.upstream.String())
}
