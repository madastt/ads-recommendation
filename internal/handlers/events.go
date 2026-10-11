package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"paw/internal/pb"

	"paw/internal/models"
)

type EventHandler struct {
	DB        *sql.DB
	MabClient pb.MabEngineClient
}

// LogEvent obsługuje POST /api/v1/events
// @Summary      Zarejestruj zdarzenie (Log)
// @Description  Zapisuje wyświetlenie (impression) lub kliknięcie (click). Endpoint publiczny - zbiera dane dla algorytmu LinUCB.
// @Tags         events
// @Accept       json
// @Produce      json
// @Param        request body models.Event true "Dane zdarzenia (ad_id, event_type: 'impression'/'click', user_context)"
// @Success      201  {object}  models.Event "Zapisane zdarzenie"
// @Failure      400  {string}  string "Brakujące dane lub niepoprawny event_type"
// @Failure      500  {string}  string "Błąd zapisu do bazy"
// @Router       /events [post]

func (h *EventHandler) LogEvent(w http.ResponseWriter, r *http.Request) {
	var req models.Event

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	)

	if err := decoder.Decode(&req); err != nil {
		http.Error(
			w,
			"Niepoprawny format JSON lub zbyt duże żądanie",
			http.StatusBadRequest,
		)
		return
	}

	if strings.TrimSpace(req.AdID) == "" ||
		(req.EventType != "impression" && req.EventType != "click") {
		http.Error(
			w,
			"Brakujące ad_id lub niepoprawny event_type",
			http.StatusBadRequest,
		)
		return
	}

	query := `
        INSERT INTO events (ad_id, event_type, user_context)
        VALUES ($1, $2, $3)
        RETURNING id, created_at
    `

	err := h.DB.QueryRowContext(
		r.Context(),
		query,
		req.AdID,
		req.EventType,
		req.UserContext,
	).Scan(&req.ID, &req.CreatedAt)
	if err != nil {
		log.Printf(
			"Błąd zapisu zdarzenia ad_id=%q: %v",
			req.AdID,
			err,
		)
		http.Error(
			w,
			"Nie udało się zapisać zdarzenia",
			http.StatusInternalServerError,
		)
		return
	}

	notification := map[string]interface{}{
		"type": "event",
		"payload": map[string]interface{}{
			"id":         req.ID,
			"ad_id":      req.AdID,
			"event_type": req.EventType,
			"created_at": req.CreatedAt,
		},
	}

	select {
	case Broadcast <- notification:
	default:
		log.Printf(
			"Pominięto powiadomienie WebSocket dla zdarzenia ID=%v",
			req.ID,
		)
	}

	if h.MabClient != nil {
		client := h.MabClient

		mabRequest := &pb.EventRequest{
			CampaignId: "frontend",
			AdId:       req.AdID,
			EventType:  req.EventType,
		}

		go func() {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				time.Second,
			)
			defer cancel()

			_, err := client.RecordEvent(ctx, mabRequest)
			if err != nil {
				log.Printf(
					"Nie udało się powiadomić silnika MAB o zdarzeniu ID=%v: %v",
					req.ID,
					err,
				)
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(req); err != nil {
		log.Printf(
			"Błąd wysyłania odpowiedzi JSON dla zdarzenia ID=%v: %v",
			req.ID,
			err,
		)
	}
}
