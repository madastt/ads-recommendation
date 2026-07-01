package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"paw/internal/models"
)

type CampaignHandler struct {
	DB *sql.DB
}

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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

func (h *CampaignHandler) GetCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`SELECT id, name, status, start_date, end_date, created_at FROM campaigns`)
	if err != nil {
		http.Error(w, "Błąd podczas pobierania danych z bazy", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

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
	json.NewEncoder(w).Encode(campaigns)
}
