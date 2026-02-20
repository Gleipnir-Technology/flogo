package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

//go:embed index.html injector.js
var embeddedFiles embed.FS

type SSEConnection struct {
	chanState chan *flogoState
	id        string
}

func (c *SSEConnection) push(w http.ResponseWriter, state *flogoState) {
	fmt.Fprintf(w, "data: {\"state\": {}, \"time\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
	w.(http.Flusher).Flush()
}

type Webserver struct {
	chanStateChange <-chan *flogoState
	connections     map[*SSEConnection]bool
}

func NewWebserver(stateChange <-chan *flogoState) *Webserver {
	return &Webserver{
		chanStateChange: stateChange,
		connections:     make(map[*SSEConnection]bool, 0),
	}
}
func (web *Webserver) Start(ctx context.Context, bind string, upstream url.URL) {
	logger := log.Ctx(ctx)
	r := chi.NewRouter()

	//r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Serve the embedded index.html for the root route
	r.Get("/.flogo", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, embeddedFiles, "index.html", "text/html")
	})
	r.Get("/.flogo/injector.js", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, embeddedFiles, "injector.js", "application/javascript")
	})

	// Handle Server-Sent Events
	r.Get("/.flogo/events", web.sseHandler)

	// Catch-all route to serve the HTML for any other path
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}))

	go web.fanoutStateChanges(ctx)
	logger.Info().Str("bind", bind).Msg("webserver starting")
	http.ListenAndServe(bind, r)
}

func (web *Webserver) fanoutStateChanges(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case state := <-web.chanStateChange:
			for c, _ := range web.connections {
				c.chanState <- state
			}
		}
	}
}

// sseHandler handles the Server-Sent Events connection
func (web *Webserver) sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	connection := SSEConnection{
		chanState: make(chan *flogoState, 10),
		id:        fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	web.connections[&connection] = true
	// Send an initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"connected\", \"time\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
	w.(http.Flusher).Flush()

	// Keep the connection open with a ticker sending periodic events
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Use a channel to detect when the client disconnects
	done := r.Context().Done()

	// Keep connection open until client disconnects
	for {
		select {
		case <-done:
			log.Info().Msg("Client closed connection")
			return
		case t := <-ticker.C:
			// Send a heartbeat message
			fmt.Fprintf(w, "data: {\"heartbeat\": \"%s\"}\n\n", t.Format(time.RFC3339))
			w.(http.Flusher).Flush()
		case state := <-connection.chanState:
			connection.push(w, state)
		}
	}
}

func serveFile(w http.ResponseWriter, files embed.FS, filename string, content_type string) {
	content, err := files.ReadFile(filename)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not load HTML from %s\n", filename), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", content_type)
	w.Write(content)
}
