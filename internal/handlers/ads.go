package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"paw/internal/models"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type AdHandler struct {
	DB *sql.DB
}

// CreateAd obsługuje POST /api/v1/ads
// @Summary      Dodaj reklamę z banerem
// @Description  Wgrywa plik graficzny i przypisuje go do kampanii. Zwraca URL obrazka. Wymaga autoryzacji JWT.
// @Tags         ads
// @Accept       multipart/form-data
// @Produce      json
// @Param        campaign_id formData string true "UUID przypisanej kampanii"
// @Param        context_features formData string true "Cechy kontekstowe JSON (np. celowana grupa wiekowa, urządzenia)"
// @Param        image formData file true "Plik graficzny banera (.png, .jpg)"
// @Success      201  {object}  models.Ad "Utworzona reklama ze ścieżką do pliku"
// @Failure      400  {string}  string "Brakujące pola lub plik jest zbyt duży"
// @Failure      500  {string}  string "Błąd zapisu na dysku lub w bazie"
// @Security     BearerAuth
// @Router       /ads [post]
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
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

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
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {

		}
	}(dst)
	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Błąd podczas zapisu zawartości pliku na dysk", http.StatusInternalServerError)
		return
	}

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
	err = json.NewEncoder(w).Encode(ad)
	if err != nil {
		return
	}
}

// GetAdsByCampaign obsługuje GET /api/v1/campaigns/{id}/ads
// @Summary      Pobierz reklamy dla kampanii
// @Description  Zwraca wszystkie banery przypisane do konkretnego UUID kampanii. Wymaga autoryzacji JWT.
// @Tags         ads
// @Produce      json
// @Param        id path string true "UUID Kampanii"
// @Success      200  {array}   models.Ad "Lista reklam"
// @Failure      400  {string}  string "Brakujące ID kampanii"
// @Failure      500  {string}  string "Błąd pobierania danych z bazy"
// @Security     BearerAuth
// @Router       /campaigns/{id}/ads [get]
func (h *AdHandler) GetAdsByCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		http.Error(w, "Brakujące ID kampanii", http.StatusBadRequest)
		return
	}

	query := `SELECT id, campaign_id, image_url, context_features, created_at FROM ads WHERE campaign_id = $1`
	rows, err := h.DB.Query(query, campaignID)
	if err != nil {
		http.Error(w, "Błąd podczas pobierania reklam z bazy", http.StatusInternalServerError)
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var ads []models.Ad
	for rows.Next() {
		var ad models.Ad
		err := rows.Scan(&ad.ID, &ad.CampaignID, &ad.ImageURL, &ad.ContextFeatures, &ad.CreatedAt)
		if err != nil {
			http.Error(w, "Błąd mapowania danych", http.StatusInternalServerError)
			return
		}
		ads = append(ads, ad)
	}

	if ads == nil {
		ads = []models.Ad{}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(ads)
	if err != nil {
		return
	}
}

// DeleteAd obsługuje DELETE /api/v1/ads/{id}
// @Summary      Usuń reklamę i jej statystyki
// @Description  Usuwa powiązane zdarzenia, rekord z bazy oraz fizyczny plik z dysku. Wymaga autoryzacji JWT.
// @Tags         ads
// @Param        id path string true "UUID Reklamy"
// @Success      204  "Pomyślnie usunięto"
// @Failure      400  {string}  string "Brakujące ID reklamy"
// @Failure      404  {string}  string "Reklama nie istnieje"
// @Failure      500  {string}  string "Błąd serwera"
// @Security     BearerAuth
// @Router       /ads/{id} [delete]
func (h *AdHandler) DeleteAd(w http.ResponseWriter, r *http.Request) {
	adID := chi.URLParam(r, "id")
	if adID == "" {
		http.Error(w, "Brakujące ID reklamy", http.StatusBadRequest)
		return
	}

	var imageURL string
	err := h.DB.QueryRow("SELECT image_url FROM ads WHERE id = $1", adID).Scan(&imageURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Reklama nie istnieje", http.StatusNotFound)
			return
		}
		http.Error(w, "Błąd podczas odczytu z bazy danych", http.StatusInternalServerError)
		return
	}

	_, err = h.DB.Exec("DELETE FROM events WHERE ad_id = $1", adID)
	if err != nil {
		http.Error(w, "Błąd podczas czyszczenia powiązanych statystyk", http.StatusInternalServerError)
		return
	}

	_, err = h.DB.Exec("DELETE FROM ads WHERE id = $1", adID)
	if err != nil {
		http.Error(w, "Błąd podczas usuwania reklamy z bazy", http.StatusInternalServerError)
		return
	}

	parts := strings.Split(imageURL, "/")
	fileName := parts[len(parts)-1]

	if fileName != "" {
		filePath := filepath.Join("uploads", fileName)

		_ = os.Remove(filePath)
	}

	w.WriteHeader(http.StatusNoContent)
}
