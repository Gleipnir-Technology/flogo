package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Start the web server
	go startServer()
	
	// Start the file watcher
	setupWatcher()
	
	// Keep the main goroutine running
	select {}
}

func startServer() {
	r := chi.NewRouter()
	
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	
	fmt.Println("Server starting on :3000")
	http.ListenAndServe(":3000", r)
}

func setupWatcher() {
	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	
	// Start watching in a goroutine
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
					
					fmt.Printf("Go file modified: %s\n", event.Name)
					buildProject()
				}
				
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
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
		log.Fatal(err)
	}
	
	fmt.Println("Watcher started. Monitoring for changes...")
	
	// Do an initial build
	buildProject()
}

func buildProject() {
	fmt.Println("Building project...")
	
	cmd := exec.Command("go", "build", ".")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		fmt.Println("Build failed:")
		fmt.Println(string(output))
	} else {
		fmt.Println("Build succeeded!")
	}
}
