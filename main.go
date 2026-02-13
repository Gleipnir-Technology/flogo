package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gdamore/tcell/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	upstreamURL      *url.URL
	isCompiling      bool
	lastBuildOutput  string
	lastBuildSuccess bool
	uiMutex          sync.Mutex
	screen           tcell.Screen
)

type UIState struct {
	isCompiling      bool
	lastBuildOutput  string
	lastBuildSuccess bool
}

func main() {
	// Initialize tcell screen
	var err error
	screen, err = tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}
	defer screen.Fini()

	// Set default style and clear screen
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	screen.SetStyle(defStyle)
	screen.Clear()

	// Get the upstream URL from environment variable
	upstream := os.Getenv("FLOGO_UPSTREAM")
	if upstream == "" {
		upstream = "http://localhost:8080" // Default if not specified
	}

	upstreamURL, err = url.Parse(upstream)
	if err != nil {
		log.Fatalf("Invalid FLOGO_UPSTREAM URL: %v", err)
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the UI in a goroutine
	go runUI(ctx)

	// Start the web server
	go startServer()
	
	// Start the file watcher
	go setupWatcher()
	
	// Handle keyboard input
	for {
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

func startServer() {
	r := chi.NewRouter()
	
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// Use the proxy handler for all routes
	r.HandleFunc("/*", proxyHandler)
	
	// Log to the console instead of drawing to the UI directly
	log.Println("Flogo server starting on :3000")
	http.ListenAndServe(":3000", r)
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
					
					log.Printf("Go file modified: %s\n", event.Name)
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
	
	log.Println("Watcher started. Monitoring for changes...")
	
	// Do an initial build
	buildProject()
}

func buildProject() {
	uiMutex.Lock()
	isCompiling = true
	uiMutex.Unlock()
	
	log.Println("Building project...")
	
	cmd := exec.Command("go", "build", ".")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	uiMutex.Lock()
	isCompiling = false
	lastBuildOutput = outputStr
	lastBuildSuccess = (err == nil)
	uiMutex.Unlock()
	
	if err != nil {
		log.Println("Build failed:")
		log.Println(outputStr)
	} else {
		log.Println("Build succeeded!")
	}
}
