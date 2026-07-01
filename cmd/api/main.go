package main

import (
	"fmt"
	"log"
	"net/http"

	"paw/internal/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db := database.InitDB()

	defer db.Close()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Serwer AdTech działa poprawnie!"))
	})

	port := ":8080"
	fmt.Printf("Uruchamianie serwera na porcie %s...\n", port)

	err := http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("Krytyczny błąd serwera: %v", err)
	}
}
