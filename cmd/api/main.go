package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"paw/internal/pb"
	"time"

	"paw/internal/database"
	"paw/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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

	mabConn, err := grpc.NewClient("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Krytyczny błąd: nie udało się zainicjalizować klienta gRPC: %v", err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Błąd podczas zamykania połączenia gRPC: %v\n", err)
		}
	}(mabConn)

	mabClient := pb.NewMabEngineClient(mabConn)

	hydrateMABState(db, mabClient)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	campaignHandler := &handlers.CampaignHandler{DB: db}
	adHandler := &handlers.AdHandler{
		DB:        db,
		MabClient: mabClient,
	}
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
		r.Get("/public/campaigns/{id}/ads", adHandler.GetPublicAdDecision)
		r.Group(func(r chi.Router) {
			r.Use(handlers.JWTMiddleware)

			r.Post("/campaigns", campaignHandler.CreateCampaign)
			r.Get("/campaigns", campaignHandler.GetCampaigns)
			r.Put("/campaigns/{id}", campaignHandler.UpdateCampaign)
			r.Get("/campaigns/{id}/ads", adHandler.GetAdsByCampaign)
			r.Get("/campaigns/{id}/stats", campaignHandler.GetCampaignStats)
			r.Post("/ads", adHandler.CreateAd)
			r.Delete("/ads/{id}", adHandler.DeleteAd)
		})
	})

	port := ":8080"
	fmt.Printf("Uruchamianie serwera na porcie %s...\n", port)

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("Krytyczny błąd serwera: %v", err)
	}

}
func hydrateMABState(db *sql.DB, client pb.MabEngineClient) {
	log.Println("Rozpoczynanie synchronizacji stanu z silnikiem MAB...")

	impressions := make(map[string]int32)
	clicks := make(map[string]int32)

	query := `SELECT ad_id, event_type, COUNT(*) FROM events GROUP BY ad_id, event_type`
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Błąd pobierania statystyk do synchronizacji: %v", err)
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	for rows.Next() {
		var adID, eventType string
		var count int32
		if err := rows.Scan(&adID, &eventType, &count); err != nil {
			continue
		}
		if eventType == "impression" {
			impressions[adID] = count
		} else if eventType == "click" {
			clicks[adID] = count
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.SyncState(ctx, &pb.SyncRequest{
		Impressions: impressions,
		Clicks:      clicks,
	})

	if err != nil {
		log.Printf("⚠️ Silnik MAB jest niedostępny. Uruchomi się z pustą pamięcią RAM (Błąd: %v)", err)
	} else {
		log.Printf("✅ Pomyślnie zsynchronizowano AI z bazą PostgreSQL! Odpowiedź Pythona: %s", resp.Message)
	}
}
