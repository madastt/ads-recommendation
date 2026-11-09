package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"paw/docs"
	localmw "paw/internal/middleware"
	"paw/internal/pb"

	"paw/internal/database"
	"paw/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	docs.SwaggerInfo.Host = "localhost:8443"
	docs.SwaggerInfo.Schemes = []string{"https"}
	db := database.InitDB()

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("Błąd podczas zamykania połączenia z bazą danych: %v\n", err)
		}
	}(db)

	mabConn, err := grpc.NewClient("mab-engine:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	const maxSyncAttempts = 10

	var syncErr error

	for attempt := 1; attempt <= maxSyncAttempts; attempt++ {
		syncErr = hydrateMABState(db, mabClient)

		if syncErr == nil {
			break
		}

		log.Printf(
			"Synchronizacja MAB nie powiodła się (próba %d/%d): %v",
			attempt,
			maxSyncAttempts,
			syncErr,
		)

		if attempt < maxSyncAttempts {
			time.Sleep(time.Second)
		}
	}

	if syncErr != nil {
		log.Fatalf(
			"Nie udało się zsynchronizować modelu po %d próbach: %v",
			maxSyncAttempts,
			syncErr,
		)
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	statsHandler := &handlers.StatsHandler{DB: db}

	campaignHandler := &handlers.CampaignHandler{DB: db}
	adHandler := &handlers.AdHandler{
		DB:        db,
		MabClient: mabClient,
	}
	eventHandler := &handlers.EventHandler{
		DB:        db,
		MabClient: mabClient,
	}
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
		r.Get("/ws", handlers.HandleWebSocket)

		r.Group(func(r chi.Router) {
			r.Use(localmw.APILogger(db))

			r.Post("/auth/login", authHandler.Login)
			r.Post("/events", eventHandler.LogEvent)
			r.Get("/public/campaigns/active", campaignHandler.GetActiveCampaign)
			r.Get("/public/campaigns/{id}/ads", adHandler.GetPublicAdDecision)

			r.Group(func(r chi.Router) {
				r.Use(handlers.JWTMiddleware)
				r.Get("/admin/stats/api", statsHandler.GetAPIStats)
				r.Post("/campaigns", campaignHandler.CreateCampaign)
				r.Get("/campaigns", campaignHandler.GetCampaigns)
				r.Put("/campaigns/{id}", campaignHandler.UpdateCampaign)
				r.Delete("/campaigns/{id}", campaignHandler.DeleteCampaign)
				r.Get("/campaigns/{id}/ads", adHandler.GetAdsByCampaign)
				r.Get("/campaigns/{id}/stats", campaignHandler.GetCampaignStats)
				r.Post("/ads", adHandler.CreateAd)
				r.Delete("/ads/{id}", adHandler.DeleteAd)
				r.Get("/ads/{id}/click-stats", adHandler.GetAdClickStats)
			})
		})
	})

	port := ":8080"
	fmt.Printf("Uruchamianie serwera na porcie %s...\n", port)

	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("Krytyczny błąd serwera: %v", err)
	}
}

func hydrateMABState(db *sql.DB, client pb.MabEngineClient) error {
	log.Println("Rozpoczynanie synchronizacji stanu z silnikiem MAB...")

	adContexts := make(map[string]string)

	// 1. Pobranie kontekstów wszystkich reklam.
	adRows, err := db.Query(`
        SELECT id, COALESCE(context_features::text, '{}')
        FROM ads
    `)
	if err != nil {
		return fmt.Errorf("błąd pobierania kontekstów reklam: %w", err)
	}
	defer adRows.Close()

	for adRows.Next() {
		var adID, contextFeatures string

		if err := adRows.Scan(&adID, &contextFeatures); err != nil {
			return fmt.Errorf("błąd odczytu kontekstu reklamy: %w", err)
		}

		adContexts[adID] = contextFeatures
	}

	if err := adRows.Err(); err != nil {
		return fmt.Errorf("błąd iterowania po reklamach: %w", err)
	}

	// 2. Wyczyszczenie modelu i inicjalizacja reklam.
	if err := syncMABState(client, &pb.SyncRequest{
		AdContexts: adContexts,
		ReplaceAll: true,
	}); err != nil {
		return fmt.Errorf("błąd inicjalizacji modelu: %w", err)
	}

	// 3. Pobranie historii zdarzeń wraz z kontekstem użytkownika.
	eventRows, err := db.Query(`
        SELECT
            ad_id,
            event_type,
            COALESCE(user_context::text, '{}')
        FROM events
        ORDER BY created_at ASC, id ASC
    `)
	if err != nil {
		return fmt.Errorf("błąd pobierania historii zdarzeń: %w", err)
	}
	defer eventRows.Close()

	const batchSize = 500

	batch := make([]*pb.EventRequest, 0, batchSize)
	restoredEvents := 0

	// 4. Wysyłanie zdarzeń do Pythona partiami.
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		err := syncMABState(client, &pb.SyncRequest{
			History: batch,
		})
		if err != nil {
			return err
		}

		restoredEvents += len(batch)
		batch = make([]*pb.EventRequest, 0, batchSize)

		return nil
	}

	for eventRows.Next() {
		var adID, eventType, userContext string

		if err := eventRows.Scan(
			&adID,
			&eventType,
			&userContext,
		); err != nil {
			return fmt.Errorf("błąd odczytu zdarzenia: %w", err)
		}

		batch = append(batch, &pb.EventRequest{
			AdId:        adID,
			EventType:   eventType,
			UserContext: userContext,
		})

		if len(batch) >= batchSize {
			if err := flushBatch(); err != nil {
				return fmt.Errorf(
					"błąd synchronizacji partii zdarzeń: %w",
					err,
				)
			}
		}
	}

	if err := eventRows.Err(); err != nil {
		return fmt.Errorf("błąd iterowania po historii zdarzeń: %w", err)
	}

	if err := flushBatch(); err != nil {
		return fmt.Errorf(
			"błąd synchronizacji ostatniej partii: %w",
			err,
		)
	}

	log.Printf(
		"Zsynchronizowano model LinUCB: %d reklam, %d zdarzeń",
		len(adContexts),
		restoredEvents,
	)

	return nil
}

func syncMABState(
	client pb.MabEngineClient,
	request *pb.SyncRequest,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	response, err := client.SyncState(ctx, request)
	if err != nil {
		return err
	}

	if !response.GetSuccess() {
		return fmt.Errorf(
			"silnik MAB odrzucił synchronizację: %s",
			response.GetMessage(),
		)
	}

	return nil
}
