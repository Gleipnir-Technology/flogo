package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

type Watcher struct {
	OnBuild chan<- struct{}
	OnDeath chan<- error
	Target  string
}

func (w Watcher) Run(ctx context.Context) {
	// Create a new watcher
	logger := log.Ctx(ctx)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.OnDeath <- fmt.Errorf("Failed to create new watcher: %w", err)
		return
	}

	// Recursively add directories to watch
	err = filepath.Walk(w.Target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		/*
			if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
				return filepath.SkipDir
			}
		*/

		// Add directories to watch
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		w.OnDeath <- fmt.Errorf("Failed to walk filepath: %w", err)
		return
	}

	logger.Info().Str("target", w.Target).Msg("Watcher started. Monitoring for changes...")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				w.OnDeath <- fmt.Errorf("Failed to get file watcher event")
				return
			}

			// Check if it's a .go file and if it was modified, created, or renamed
			if filepath.Ext(event.Name) == ".go" &&
				(event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Rename == fsnotify.Rename) {

				typestring := eventToString(event)
				logger.Info().Str("name", event.Name).Str("type", typestring).Msg("notify event")

				emitBuildSignal(ctx)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				w.OnDeath <- fmt.Errorf("Failed to get file watcher errors")
			} else {
				w.OnDeath <- fmt.Errorf("Got watcher error: %w", err)
			}
			return
		}
	}
}

func emitBuildSignal(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("fake build")
}

var eventTypeToSymbol = map[fsnotify.Op]string{
	fsnotify.Create: "C",
	fsnotify.Write:  "W",
	fsnotify.Remove: "D",
	fsnotify.Rename: "R",
}

func eventToString(event fsnotify.Event) string {
	var sb strings.Builder

	for k, v := range eventTypeToSymbol {
		if event.Has(k) {
			sb.WriteString(v)
		} else {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}
