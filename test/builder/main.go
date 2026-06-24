package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/Gleipnir-Technology/flogo/state"
	"github.com/Gleipnir-Technology/flogo/ui"
)

func main() {
	ctx := context.Background()

	// chanOnBuilder:   make(chan EventBuilder),
	builder := Builder{
		Debounce: time.Millisecond * 300,
		OnEvent:  mgr.chanOnBuilder,
		Target:   target,
		ToBuild:  mgr.chanDoBuilder,
	}
	on_ui := make(chan ui.Event)
	do_ui := make(chan *state.Flogo)
	// Start the UI in a goroutine
	go func() {
		err := u.Run(ctx, on_ui, do_ui)
		if err != nil {
			fmt.Printf("ui run: %v", err)
			os.Exit(3)
		}
	}()
	defer u.Close()

	ticker := time.NewTicker(1 * time.Second)
	counter := 0
	state := &state.Flogo{
		Builder: &state.Builder{
			Status: state.StatusBuilderOK,
		},
		Runner: &state.Runner{
			RunCurrent: &state.Process{
				Output: []byte{},
			},
			Status: state.StatusRunnerRunning,
		},
	}
	is_running := true
	for is_running {
		select {
		case <-ticker.C:
			counter++
			state.Runner.RunCurrent.Output = fmt.Appendf(state.Runner.RunCurrent.Output, "%d", counter)
			do_ui <- state
		case evt := <-on_ui:
			switch evt.Type {
			case ui.EventExit:
				is_running = false
			}
		}
	}
}
