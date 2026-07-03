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
	err = json.NewEncoder(w).Encode(req)
	if err != nil {
		return
	}
	go func(event models.Event) {
		Broadcast <- event
	}(req)
}
