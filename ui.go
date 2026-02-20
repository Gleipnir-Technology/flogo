package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	//"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

type ui struct {
	screen tcell.Screen
	state  uiState
	target string
}
type uiState struct {
	isCompiling      bool
	isRunning        bool
	lastBuildOutput  string
	lastBuildSuccess bool
	lastRunStdout    string
	lastRunStderr    string
}

func newUI(target string) (*ui, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return nil, fmt.Errorf("failed to init screen: %w", err)
	}
	screen.Clear()
	return &ui{
		screen: screen,
		state: uiState{
			isCompiling:      false,
			isRunning:        false,
			lastBuildOutput:  "no output",
			lastBuildSuccess: true,
		},
		target: target,
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
		log.Fatalf("%+v", err)
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(color.Reset).Foreground(color.Reset)
	u.screen.SetStyle(defStyle)

	// Clear screen
	u.screen.Clear()
}
func (u ui) Sync() {
	u.drawUI()
}

var (
	isCompiling bool
)

func (u ui) handleInput() {
	/*
		ev := u.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return
			}
		case *tcell.EventResize:
			u.screen.Sync()
		}
	*/
}
func runUI(ctx context.Context) {
	/*
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				uiMutex.Lock()
				state := UIState{
					isCompiling:      isCompiling,
					lastBuildOutput:  lastBuildOutput,
					lastBuildSuccess: lastBuildSuccess,
				}
				uiMutex.Unlock()

				u.drawUI(state)
			}
		}
	*/
}

func (u ui) drawUI() {
	u.screen.Clear()

	// Draw title
	u.drawTitle()
	// Draw upstream info
	//u.drawText(0, 1, tcell.StyleDefault.Foreground(tcell.ColorYellow), fmt.Sprintf("Upstream: %s", upstreamURL.String()))

	if !u.state.lastBuildSuccess && u.state.lastBuildOutput != "" {
		u.drawBuildFailure()
	} else if u.state.isCompiling {
		u.drawCompilation()
	} else if u.state.isRunning {
		u.drawRunning()
	} else {
		u.drawUnknown()
	}

	u.screen.Show()
}

func (u ui) drawBuildFailure() {
	u.drawStatus("Build Failed. Errors:", tcell.StyleDefault.Foreground(tcell.ColorRed))

	// Split output into lines and display them
	lines := strings.Split(u.state.lastBuildOutput, "\n")
	for i, line := range lines {
		if i < 15 { // Limit number of lines to avoid overflow
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), line)
		} else if i == 15 {
			u.drawText(1, 3+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), "... (more errors)")
			break
		}
	}
}

func (u ui) drawCompilation() {
}
func (u ui) drawRunning() {
	if u.state.lastRunStderr != "" {
		u.drawText(0, 1, tcell.StyleDefault.Foreground(tcell.ColorYellow), "stderr:")
		u.drawTextMultiline(0, 2, tcell.StyleDefault.Foreground(tcell.ColorWhite), u.state.lastRunStderr)
	} else if u.state.lastRunStdout != "" {
		u.drawText(0, 1, tcell.StyleDefault.Foreground(tcell.ColorGreen), "stdout:")
		u.drawTextMultiline(0, 2, tcell.StyleDefault.Foreground(tcell.ColorWhite), u.state.lastRunStdout)
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
func (u ui) drawTextMultiline(x, y int, style tcell.Style, text string) {
	// Split output into lines and display them
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		u.drawText(x, y+i, style, line)
	}
}
func (u ui) drawTitle() {
	if u.state.isCompiling {
		u.drawText(0, 0, tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true), "Compiling")
	} else {
		u.drawText(0, 0, tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true), "Idle")
	}

	if u.state.isRunning {
		u.drawText(10, 0, tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true), "Running")
	} else {
		u.drawText(10, 0, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true), "Dead")
	}
}
func (u ui) drawUnknown() {
	u.drawStatus("Unknown.", tcell.StyleDefault.Foreground(tcell.ColorRed))
}
