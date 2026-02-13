package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed index.html
var indexHTML embed.FS

func startServer(bind string) {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Serve the embedded index.html for all routes
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
