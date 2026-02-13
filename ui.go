package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

var (
	isCompiling bool
	screen      tcell.Screen
)

type UIState struct {
	isCompiling      bool
	lastBuildOutput  string
	lastBuildSuccess bool
}

func initUI() {
	// Initialize tcell screen
	var err error
	screen, err = tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}

	// Set default style and clear screen
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	screen.SetStyle(defStyle)
	screen.Clear()

}

func handleInput() {
	ev := screen.PollEvent()
	switch ev := ev.(type) {
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
			return
		}
	case *tcell.EventResize:
		screen.Sync()
	}
}
func runUI(ctx context.Context) {
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

			drawUI(state)
		}
	}
}

func drawUI(state UIState) {
	screen.Clear()

	// Draw title
	drawText(0, 0, tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true), "FLOGO - Go Web Development Tool")
	drawText(0, 1, tcell.StyleDefault.Foreground(tcell.ColorWhite), "Press ESC or Ctrl+C to exit")

	// Draw upstream info
	drawText(0, 3, tcell.StyleDefault.Foreground(tcell.ColorYellow), fmt.Sprintf("Upstream: %s", upstreamURL.String()))

	// Draw status
	statusStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	statusText := "Idle"

	if state.isCompiling {
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
		statusText = "Compiling..."
	} else if !state.lastBuildSuccess && state.lastBuildOutput != "" {
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorRed)
		statusText = "Build Failed"
	}

	drawText(0, 5, statusStyle.Bold(true), fmt.Sprintf("Status: %s", statusText))

	// Draw last build output if there was an error
	if !state.lastBuildSuccess && state.lastBuildOutput != "" {
		drawText(0, 7, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true), "Build Errors:")

		// Split output into lines and display them
		lines := strings.Split(state.lastBuildOutput, "\n")
		for i, line := range lines {
			if i < 15 { // Limit number of lines to avoid overflow
				drawText(2, 8+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), line)
			} else if i == 15 {
				drawText(2, 8+i, tcell.StyleDefault.Foreground(tcell.ColorWhite), "... (more errors)")
				break
			}
		}
	}

	screen.Show()
}

func drawText(x, y int, style tcell.Style, text string) {
	for i, r := range text {
		screen.SetContent(x+i, y, r, nil, style)
	}
}
