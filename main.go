package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	
	// Add some middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// Define routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	
	// Start the server
	fmt.Println("Server starting on :10000")
	http.ListenAndServe(":10000", r)
}
