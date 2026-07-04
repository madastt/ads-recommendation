package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"paw/internal/database"
	"paw/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "paw/docs"
)

// @title           AdTech MAB Optimization API
// @version         1.0
// @description     REST API serwera dla systemu optymalizacji reklam w oparciu o algorytmy Multi-Armed Bandit.
// @termsOfService  http://swagger.io/terms/

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apiKey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Wpisz token w formacie: Bearer <token_jwt>
//
// UWAGA: API udostępnia również publiczny kanał WebSocket pod adresem ws://localhost:8080/api/v1/ws
func main() {
	db := database.InitDB()

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("Błąd podczas zamykania połączenia z bazą danych: %v\n", err)
		}
	}(db)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	campaignHandler := &handlers.CampaignHandler{DB: db}
	adHandler := &handlers.AdHandler{DB: db}
	eventHandler := &handlers.EventHandler{DB: db}
	authHandler := &handlers.AuthHandler{DB: db}

	go handlers.HandleMessages()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Serwer AdTech działa poprawnie!"))
		if err != nil {
			log.Printf("Błąd podczas wysyłania odpowiedzi healthcheck: %v\n", err)
			return
		}
	})

	fs := http.FileServer(http.Dir("./uploads"))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", authHandler.Login)
		r.Post("/events", eventHandler.LogEvent)
		r.Get("/ws", handlers.HandleWebSocket)
		r.Get("/public/campaigns/{id}/ads", adHandler.GetAdsByCampaign)
		r.Group(func(r chi.Router) {
			r.Use(handlers.JWTMiddleware)

			r.Post("/campaigns", campaignHandler.CreateCampaign)
			r.Get("/campaigns", campaignHandler.GetCampaigns)
			r.Get("/campaigns/{id}/ads", adHandler.GetAdsByCampaign)
			r.Get("/campaigns/{id}/stats", campaignHandler.GetCampaignStats)
			r.Post("/ads", adHandler.CreateAd)
			r.Delete("/ads/{id}", adHandler.DeleteAd)
		})
	})

	port := ":8080"
	fmt.Printf("Uruchamianie serwera na porcie %s...\n", port)

	err := http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("Krytyczny błąd serwera: %v", err)
	}
}
