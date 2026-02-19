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
}
type uiState struct {
	isCompiling      bool
	lastBuildOutput  string
	lastBuildSuccess bool
}

func newUI() (*ui, error) {
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
			lastBuildOutput:  "no output",
			lastBuildSuccess: true,
		},
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
	u.drawText(0, 0, tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true), "FLOGO - Go Web Development Tool")
	u.drawText(0, 1, tcell.StyleDefault.Foreground(tcell.ColorWhite), "Press ESC or Ctrl+C to exit")

	// Draw upstream info
	u.drawText(0, 3, tcell.StyleDefault.Foreground(tcell.ColorYellow), fmt.Sprintf("Upstream: %s", upstreamURL.String()))

	// Draw status
	statusStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	statusText := "Idle"

	if u.state.isCompiling {
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
		statusText = "Compiling..."
	} else if !u.state.lastBuildSuccess && u.state.lastBuildOutput != "" {
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorRed)
		statusText = "Build Failed"
	}

	u.drawText(0, 5, statusStyle.Bold(true), fmt.Sprintf("Status: %s", statusText))

	// Draw last build output if there was an error
	if !u.state.lastBuildSuccess && u.state.lastBuildOutput != "" {
		u.drawText(0, 7, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true), "Build Errors:")

		// Split output into lines and display them
		lines := strings.Split(u.state.lastBuildOutput, "\n")
		for i, line := range lines {
			if i < 15 { // Limit number of lines to avoid overflow
				u.drawText(2, 8+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), line)
			} else if i == 15 {
				u.drawText(2, 8+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), "... (more errors)")
				break
			}
		}
	}

	u.screen.Show()
}

func (u ui) drawText(x, y int, style tcell.Style, text string) {
	for i, r := range text {
		u.screen.SetContent(x+i, y, r, nil, style)
	}
}
