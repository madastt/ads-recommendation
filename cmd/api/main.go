package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"paw/internal/handlers"

	"paw/internal/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db := database.InitDB()

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Serwer AdTech działa poprawnie!"))
		if err != nil {
			return
		}
	})

	campaignHandler := &handlers.CampaignHandler{DB: db}
	adHandler := &handlers.AdHandler{DB: db}
	eventHandler := &handlers.EventHandler{DB: db}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Serwer AdTech działa poprawnie!"))
		if err != nil {
			return
		}
	})

	fs := http.FileServer(http.Dir("./uploads"))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/campaigns", campaignHandler.CreateCampaign)
		r.Get("/campaigns", campaignHandler.GetCampaigns)
		r.Get("/campaigns/{id}/ads", adHandler.GetAdsByCampaign)
		r.Post("/ads", adHandler.CreateAd)
		r.Post("/events", eventHandler.LogEvent)
	})

	port := ":8080"
	fmt.Printf("Uruchamianie serwera na porcie %s...\n", port)

	err := http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("Krytyczny błąd serwera: %v", err)
	}
}
