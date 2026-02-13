package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed index.html
var indexHTML embed.FS

func startServer(bind string) {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Serve the embedded index.html for the root route
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := indexHTML.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not load HTML", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	// Handle Server-Sent Events
	r.Get("/events", sseHandler)

	// Catch-all route to serve the HTML for any other path
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		content, err := indexHTML.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not load HTML", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	log.Printf("Server starting on %s", bind)
	http.ListenAndServe(bind, r)
}

// sseHandler handles the Server-Sent Events connection
func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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
			log.Println("Client closed connection")
			return
		case t := <-ticker.C:
			// Send a heartbeat message
			fmt.Fprintf(w, "data: {\"heartbeat\": \"%s\"}\n\n", t.Format(time.RFC3339))
			w.(http.Flusher).Flush()
		}
	}
}
