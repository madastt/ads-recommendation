package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"paw/internal/models"

	"github.com/go-chi/chi/v5"
)

type CampaignHandler struct {
	DB *sql.DB
}

// CreateCampaign obsługuje POST /api/v1/campaigns
// @Summary      Utwórz nową kampanię
// @Description  Dodaje nową kampanię reklamową. Wymaga ważnego tokena JWT.
// @Tags         campaigns
// @Accept       json
// @Produce      json
// @Param        request body models.Campaign true "Dane kampanii (name, start_date, end_date)"
// @Success      201  {object}  models.Campaign "Utworzona kampania"
// @Failure      400  {string}  string "Niepoprawny format JSON"
// @Failure      500  {string}  string "Błąd zapisu do bazy"
// @Security     BearerAuth
// @Router       /campaigns [post]
func (h *CampaignHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req models.Campaign
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Niepoprawny format JSON", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO campaigns (name, status, start_date, end_date) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, created_at`

	err = h.DB.QueryRow(query, req.Name, "active", req.StartDate, req.EndDate).Scan(&req.ID, &req.CreatedAt)
	if err != nil {
		http.Error(w, "Błąd podczas zapisu do bazy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req.Status = "active"

	Broadcast <- map[string]interface{}{
		"type":    "campaign_created",
		"payload": req,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(req)
	if err != nil {
		return
	}
}

// GetCampaigns obsługuje GET /api/v1/campaigns
// @Summary      Pobierz listę kampanii
// @Description  Zwraca wszystkie kampanie reklamowe zarejestrowane w bazie danych PostgreSQL.
// @Tags         campaigns
// @Produce      json
// @Success      200  {array}   models.Campaign
// @Failure      500  {string}  string "Błąd wewnętrzny serwera"
// @Security     BearerAuth
// @Router       /campaigns [get]
func (h *CampaignHandler) GetCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`SELECT id, name, status, start_date, end_date, created_at FROM campaigns`)
	if err != nil {
		http.Error(w, "Błąd podczas pobierania danych z bazy", http.StatusInternalServerError)
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var campaigns []models.Campaign
	for rows.Next() {
		var c models.Campaign
		err := rows.Scan(&c.ID, &c.Name, &c.Status, &c.StartDate, &c.EndDate, &c.CreatedAt)
		if err != nil {
			http.Error(w, "Błąd mapowania danych", http.StatusInternalServerError)
			return
		}
		campaigns = append(campaigns, c)
	}

	if campaigns == nil {
		campaigns = []models.Campaign{}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(campaigns)
	if err != nil {
		return
	}
}

// GetActiveCampaign obsługuje GET /api/v1/public/campaigns/active
// @Summary      Pobierz aktualnie aktywną kampanię (publiczne)
// @Description  Zwraca najnowszą kampanię ze statusem "active". Używane przez sklep, żeby automatycznie
// @Description  wyświetlać reklamy z bieżącej kampanii bez znajomości jej UUID.
// @Tags         campaigns
// @Produce      json
// @Success      200  {object}  models.Campaign
// @Failure      404  {string}  string "Brak aktywnej kampanii"
// @Failure      500  {string}  string "Błąd wewnętrzny serwera"
// @Router       /public/campaigns/active [get]
func (h *CampaignHandler) GetActiveCampaign(w http.ResponseWriter, r *http.Request) {
	var c models.Campaign

	query := `
		SELECT id, name, status, start_date, end_date, created_at 
		FROM campaigns 
		WHERE status = 'active' 
		ORDER BY created_at DESC 
		LIMIT 1`

	err := h.DB.QueryRow(query).Scan(&c.ID, &c.Name, &c.Status, &c.StartDate, &c.EndDate, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Brak aktywnej kampanii", http.StatusNotFound)
			return
		}
		http.Error(w, "Błąd podczas pobierania aktywnej kampanii", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(c)
	if err != nil {
		return
	}
}

// GetCampaignStats obsługuje GET /api/v1/campaigns/{id}/stats
// @Summary      Pobierz statystyki kampanii
// @Description  Agreguje wyświetlenia i kliknięcia dla wszystkich reklam w kampanii i wylicza CTR. Wymaga autoryzacji JWT.
// @Tags         campaigns
// @Produce      json
// @Param        id path string true "UUID Kampanii"
// @Success      200  {array}   models.AdStats "Statystyki reklam"
// @Failure      400  {string}  string "Brakujące ID kampanii"
// @Failure      500  {string}  string "Błąd pobierania danych z bazy"
// @Security     BearerAuth
// @Router       /campaigns/{id}/stats [get]
func (h *CampaignHandler) GetCampaignStats(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, "Brakujące ID kampanii", http.StatusBadRequest)
		return
	}

	query := `
		SELECT 
			a.id AS ad_id,
			COUNT(e.id) FILTER (WHERE e.event_type = 'impression') AS impressions,
			COUNT(e.id) FILTER (WHERE e.event_type = 'click') AS clicks
		FROM ads a
		LEFT JOIN events e ON a.id = e.ad_id
		WHERE a.campaign_id = $1
		GROUP BY a.id
	`

	rows, err := h.DB.Query(query, campaignID)
	if err != nil {
		http.Error(w, "Błąd podczas agregacji statystyk w bazie", http.StatusInternalServerError)
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var statsList []models.AdStats

	for rows.Next() {
		var stat models.AdStats
		err := rows.Scan(&stat.AdID, &stat.Impressions, &stat.Clicks)
		if err != nil {
			http.Error(w, "Błąd mapowania statystyk", http.StatusInternalServerError)
			return
		}

		if stat.Impressions > 0 {
			stat.CTR = float64(stat.Clicks) / float64(stat.Impressions)
		} else {
			stat.CTR = 0.0
		}

		statsList = append(statsList, stat)
	}

	if statsList == nil {
		statsList = []models.AdStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(statsList)
	if err != nil {
		return
	}
}

// UpdateCampaign obsługuje PUT /api/v1/campaigns/{id}
// @Summary      Aktualizuj istniejącą kampanię
// @Description  Modyfikuje parametry istniejącej kampanii (nazwę oraz ramy czasowe). Wymaga ważnego tokena JWT.
// @Tags         campaigns
// @Accept       json
// @Param        id path string true "UUID Kampanii"
// @Param        request body models.Campaign true "Nowe dane kampanii (wymagane: name, start_date, end_date)"
// @Success      204  "Pomyślnie zaktualizowano (Brak zawartości)"
// @Failure      400  {string}  string "Brakujące ID kampanii lub niepoprawny format JSON"
// @Failure      404  {string}  string "Nie znaleziono kampanii do aktualizacji"
// @Failure      500  {string}  string "Błąd wewnętrzny serwera"
// @Security     BearerAuth
// @Router       /campaigns/{id} [put]
func (h *CampaignHandler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, "Brakujące ID kampanii", http.StatusBadRequest)
		return
	}
	var payload struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Niepoprawny format danych JSON", http.StatusBadRequest)
		return
	}
	query := `
       UPDATE campaigns 
       SET name = $1, start_date = $2, end_date = $3 
       WHERE id = $4`

	result, err := h.DB.Exec(query, payload.Name, payload.StartDate, payload.EndDate, campaignID)
	if err != nil {
		log.Printf("Błąd podczas aktualizacji kampanii: %v", err)
		http.Error(w, "Błąd serwera podczas zapisu zmian", http.StatusInternalServerError)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		http.Error(w, "Nie znaleziono kampanii do aktualizacji", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteCampaign obsługuje DELETE /api/v1/campaigns/{id}
// @Summary      Zarchiwizuj kampanię (Soft Delete)
// @Description  Zmienia status kampanii na 'archived', chroniąc historię statystyk reklam.
// @Tags         campaigns
// @Param        id path string true "UUID Kampanii"
// @Success      204  "Pomyślnie zarchiwizowano (Brak zawartości)"
// @Failure      400  {string}  string "Brakujące ID kampanii"
// @Failure      404  {string}  string "Nie znaleziono kampanii lub już zarchiwizowana"
// @Failure      500  {string}  string "Błąd wewnętrzny serwera"
// @Security     BearerAuth
// @Router       /campaigns/{id} [delete]
func (h *CampaignHandler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, "Brakujące ID kampanii", http.StatusBadRequest)
		return
	}

	// Zamiast DELETE używamy UPDATE. Zapobiegamy też ponownej archiwizacji.
	query := `UPDATE campaigns SET status = 'archived' WHERE id = $1 AND status != 'archived'`
	result, err := h.DB.Exec(query, campaignID)

	if err != nil {
		http.Error(w, "Błąd podczas archiwizacji kampanii", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		http.Error(w, "Nie znaleziono kampanii do archiwizacji", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
