package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	upstreamURL *url.URL
)

func main() {
	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:8080" // Default if not specified
	}

	var err error
	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		log.Fatalf("Invalid FLOGO_UPSTREAM URL: %v", err)
	}

	fmt.Printf("Using upstream server: %s\n", upstreamURL.String())

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
	
	// Use the proxy handler for all routes
	r.HandleFunc("/*", proxyHandler)
	
	fmt.Println("Flogo server starting on :10000")
	http.ListenAndServe(":10000", r)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if upstream is alive
	if !isUpstreamAlive() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Upstream server is not available. Your application is either starting up or has errors."))
		return
	}
	
	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	
	// Update the headers to allow for SSL redirection
	r.URL.Host = upstreamURL.Host
	r.URL.Scheme = upstreamURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = upstreamURL.Host

	proxy.ServeHTTP(w, r)
}

func isUpstreamAlive() bool {
	client := http.Client{
		Timeout: 100 * time.Millisecond,
	}
	resp, err := client.Get(upstreamURL.String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500 // Consider any status below 500 as "alive"
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
