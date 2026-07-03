package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"paw/internal/models"
)

type EventHandler struct {
	DB *sql.DB
}

func (h *EventHandler) LogEvent(w http.ResponseWriter, r *http.Request) {
	var req models.Event

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Niepoprawny format JSON", http.StatusBadRequest)
		return
	}

	if req.AdID == "" || (req.EventType != "impression" && req.EventType != "click") {
		http.Error(w, "Brakujące ad_id lub niepoprawny event_type (tylko: 'impression', 'click')", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO events (ad_id, event_type, user_context) 
		VALUES ($1, $2, $3) 
		RETURNING id, created_at`

	err = h.DB.QueryRow(query, req.AdID, req.EventType, req.UserContext).Scan(&req.ID, &req.CreatedAt)
	if err != nil {
		http.Error(w, "Błąd podczas logowania zdarzenia: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}
