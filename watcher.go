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
	OnEvent chan<- string
	Target  string
}

func (w Watcher) Run(ctx context.Context) error {
	// Create a new watcher
	logger := log.Ctx(ctx).With().Caller().Logger()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("Failed to create new watcher: %w", err)
	}

	// Recursively add directories to watch
	abs, err := filepath.Abs(w.Target)
	if err != nil {
		return fmt.Errorf("Determine abs: %w", err)
	}
	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("creating walk: %w", err)
		}

		// Skip hidden directories and vendor
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
			logger.Debug().Str("name", info.Name()).Msg("Skipping")
			return filepath.SkipDir
		}

		// Add directories to watch
		if info.IsDir() {
			logger.Debug().Str("path", path).Msg("add to watch list")
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("Failed to walk filepath: %w", err)
	}

	logger.Info().Str("target", w.Target).Msg("Started watcher loop")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("Failed to get file watcher event")
			}

			// Check if it's a .go file and if it was modified, created, or renamed
			if filepath.Ext(event.Name) == ".go" &&
				(event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create) {
				//event.Op&fsnotify.Rename == fsnotify.Rename) {

				typestring := eventToString(event)
				logger.Debug().Str("name", event.Name).Str("type", typestring).Msg("notify event")

				go func() {
					w.OnEvent <- event.Name
				}()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("Failed to get file watcher errors: %w", err)
			} else {
				return fmt.Errorf("Got watcher error: %w", err)
			}
		}
	}
}

type opPair struct {
	Op  fsnotify.Op
	Sym string
}

var eventTypeToSymbol = []opPair{
	opPair{Op: fsnotify.Create, Sym: "C"},
	opPair{Op: fsnotify.Write, Sym: "W"},
	opPair{Op: fsnotify.Remove, Sym: "D"},
	//opPair{Op: fsnotify.Rename, Sym: "R"},
}

func eventToString(event fsnotify.Event) string {
	var sb strings.Builder

	for _, p := range eventTypeToSymbol {
		if event.Has(p.Op) {
			sb.WriteString(p.Sym)
		} else {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}
