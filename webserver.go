package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)
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

