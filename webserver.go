package main

import (
	"context"
	"embed"
	"encoding/json"
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

type MessageHeartbeat struct {
	Time time.Time `json:"time"`
}
type MessageSSE struct {
	Content interface{} `json:"content"`
	Type    string      `json:"type"`
}
type MessageStatus struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Status string `json:"status"`
}
type MessageState struct {
	BuilderStatus MessageStatus `json:"builder"`
	RunnerStatus  MessageStatus `json:"runner"`
}
type SSEConnection struct {
	chanState chan *flogoState
	id        string
}

func (c *SSEConnection) SendState(w http.ResponseWriter, state *flogoState) error {
	return send(w, MessageSSE{
		Content: MessageState{
			BuilderStatus: MessageStatus{
				Stdout: state.lastBuildOutput,
				Stderr: "",
				Status: StatusStringBuilder(state.builderStatus),
			},
			RunnerStatus: MessageStatus{
				Stdout: StripColorCodes(state.lastRunStdout),
				Stderr: StripColorCodes(state.lastRunStderr),
				Status: StatusStringRunner(state.runnerStatus),
			},
		},
		Type: "state",
	})
}
func (c *SSEConnection) SendHeartbeat(w http.ResponseWriter, t time.Time) error {
	return send(w, MessageSSE{
		Content: MessageHeartbeat{
			Time: t,
		},
		Type: "heartbeat",
	})
}
func send[T any](w http.ResponseWriter, msg T) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling json: %w", err)
	}
	// Write in SSE format: "data: <json>\n\n"
	_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
	if err != nil {
		return fmt.Errorf("writing SSE message: %w", err)
	}

	w.(http.Flusher).Flush()
	return nil
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
	var err error
	for {
		err = nil
		select {
		case <-done:
			log.Info().Msg("Client closed connection")
			return
		case t := <-ticker.C:
			// Send a heartbeat message
			err = connection.SendHeartbeat(w, t)
		case state := <-connection.chanState:
			err = connection.SendState(w, state)
		}
		if err != nil {
			log.Error().Err(err).Msg("Failed to send state from webserver")
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
