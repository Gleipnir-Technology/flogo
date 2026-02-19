package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

//go:embed index.html
var indexHTML embed.FS

func startServer(ctx context.Context, bind string, upstream url.URL) {
	logger := log.Ctx(ctx)
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Store the original response modifier
	originalModifyResponse := proxy.ModifyResponse

	// Add our JavaScript injection logic
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Call the original modifier if it exists
		if originalModifyResponse != nil {
			if err := originalModifyResponse(resp); err != nil {
				logger.Info().Err(err).Msg("failed to modify response")
				return err
			}
		}
		logger.Info().Msg("modifying response")

		// Only inject JavaScript into HTML responses
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(contentType), "text/html") {
			return nil
		}

		// Read the original body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()

		// Inject the JavaScript alert
		scriptTag := "<script>alert('hello world');</script>"

		// Try to inject before </body> if it exists, otherwise append to the HTML
		bodyStr := string(body)
		if strings.Contains(bodyStr, "</body>") {
			bodyStr = strings.Replace(bodyStr, "</body>", scriptTag+"</body>", 1)
		} else if strings.Contains(bodyStr, "</html>") {
			bodyStr = strings.Replace(bodyStr, "</html>", scriptTag+"</html>", 1)
		} else {
			bodyStr = bodyStr + scriptTag
		}

		// Update content length and body
		resp.ContentLength = int64(len(bodyStr))
		resp.Body = io.NopCloser(strings.NewReader(bodyStr))

		return nil
	}

	// Serve the embedded index.html for the root route
	/*r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := indexHTML.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not load HTML", http.StatusInternalServerError)
			return
		}

		w.Write(content)
	})*/

	// Handle Server-Sent Events
	//r.Get("/events", sseHandler)

	// Catch-all route to serve the HTML for any other path
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}))

	logger.Info().Str("bind", bind).Msg("webserver starting")
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
			log.Info().Msg("Client closed connection")
			return
		case t := <-ticker.C:
			// Send a heartbeat message
			fmt.Fprintf(w, "data: {\"heartbeat\": \"%s\"}\n\n", t.Format(time.RFC3339))
			w.(http.Flusher).Flush()
		}
	}
}
