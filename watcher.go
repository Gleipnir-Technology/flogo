package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

func runFileWatcher(ctx context.Context, something_died chan<- error, target string) {
	// Create a new watcher
	logger := log.Ctx(ctx)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		something_died <- fmt.Errorf("Failed to create new watcher: %w", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Check if it's a .go file and if it was modified, created, or renamed
				if filepath.Ext(event.Name) == ".go" &&
					(event.Op&fsnotify.Write == fsnotify.Write ||
						event.Op&fsnotify.Create == fsnotify.Create ||
						event.Op&fsnotify.Rename == fsnotify.Rename) {

					// Debounce multiple events by waiting a little
					time.Sleep(100 * time.Millisecond)

					buildProject()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error().Err(err).Msg("got errors")
			}
		}
	}()

	// Recursively add directories to watch
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
			return filepath.SkipDir
		}

		// Add directories to watch
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		something_died <- fmt.Errorf("Failed to walk filepath: %w", err)
	}

	logger.Info().Msg("Watcher started. Monitoring for changes...")
}

func buildProject() {
}
