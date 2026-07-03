package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"paw/internal/models"
)

type AdHandler struct {
	DB *sql.DB
}

func (h *AdHandler) CreateAd(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Plik jest zbyt duży", http.StatusBadRequest)
		return
	}
	campaignID := r.FormValue("campaign_id")
	contextFeatures := r.FormValue("context_features")

	if campaignID == "" {
		http.Error(w, "Brakujące id kampanii (campaign_id)", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Błąd odczytu pliku z żądania. Upewnij się, że wysyłasz pole 'image'", http.StatusBadRequest)
		return
	}
	defer file.Close()

	err = os.MkdirAll("uploads", os.ModePerm)
	if err != nil {
		http.Error(w, "Błąd krytyczny serwera przy tworzeniu folderu", http.StatusInternalServerError)
		return
	}

	fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(handler.Filename))
	filePath := filepath.Join("uploads", fileName)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Błąd przy zapisywaniu pliku na dysku", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	imageURL := fmt.Sprintf("/uploads/%s", fileName)

	query := `
		INSERT INTO ads (campaign_id, image_url, context_features) 
		VALUES ($1, $2, $3) 
		RETURNING id, created_at`

	var ad models.Ad
	err = h.DB.QueryRow(query, campaignID, imageURL, contextFeatures).Scan(&ad.ID, &ad.CreatedAt)
	if err != nil {
		http.Error(w, "Błąd podczas zapisu do bazy: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ad.CampaignID = campaignID
	ad.ImageURL = imageURL
	ad.ContextFeatures = contextFeatures

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ad)
}
