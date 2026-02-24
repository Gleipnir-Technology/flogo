package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

//go:embed index.html
var indexHTML embed.FS

func main() {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := indexHTML.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not load HTML", http.StatusInternalServerError)
			return
		}
		w.Write(content)
	})

	bind := os.Getenv("BIND")
	if bind == "" {
		bind = ":9003"
	}
	fmt.Printf("Server starting on port %s...\n", bind)
	log.Fatal(http.ListenAndServe(bind, r))
}
