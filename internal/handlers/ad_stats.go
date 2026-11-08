package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type ClickBreakdown struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type RecentClick struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	AgeGroup  string    `json:"age_group"`
	Device    string    `json:"device"`
	Interests []string  `json:"interests"`
}

type AdClickStatsResponse struct {
	TotalClicks  int              `json:"total_clicks"`
	ByAgeGroup   []ClickBreakdown `json:"by_age_group"`
	ByDevice     []ClickBreakdown `json:"by_device"`
	ByInterest   []ClickBreakdown `json:"by_interest"`
	RecentClicks []RecentClick    `json:"recent_clicks"`
}

func (h *AdHandler) GetAdClickStats(w http.ResponseWriter, r *http.Request) {
	adID := chi.URLParam(r, "id")

	if adID == "" {
		http.Error(w, "Brakujące ID reklamy", http.StatusBadRequest)
		return
	}

	var exists bool

	err := h.DB.QueryRowContext(
		r.Context(),
		`SELECT EXISTS(SELECT 1 FROM ads WHERE id = $1)`,
		adID,
	).Scan(&exists)
	if err != nil {
		http.Error(w, "Błąd sprawdzania reklamy", http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, "Reklama nie istnieje", http.StatusNotFound)
		return
	}

	result := AdClickStatsResponse{
		ByAgeGroup:   []ClickBreakdown{},
		ByDevice:     []ClickBreakdown{},
		ByInterest:   []ClickBreakdown{},
		RecentClicks: []RecentClick{},
	}

	err = h.DB.QueryRowContext(
		r.Context(),
		`
		SELECT COUNT(*)
		FROM events
		WHERE ad_id = $1 AND event_type = 'click'
		`,
		adID,
	).Scan(&result.TotalClicks)
	if err != nil {
		http.Error(w, "Błąd pobierania liczby kliknięć", http.StatusInternalServerError)
		return
	}

	var queryErr error

	result.ByAgeGroup, queryErr = getClickBreakdown(
		h.DB,
		`
		SELECT
			COALESCE(user_context->>'age_group', 'Nieznany') AS label,
			COUNT(*) AS total
		FROM events
		WHERE ad_id = $1 AND event_type = 'click'
		GROUP BY COALESCE(user_context->>'age_group', 'Nieznany')
		ORDER BY total DESC
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk wieku", http.StatusInternalServerError)
		return
	}

	result.ByDevice, queryErr = getClickBreakdown(
		h.DB,
		`
		SELECT
			COALESCE(user_context->>'device', 'Nieznany') AS label,
			COUNT(*) AS total
		FROM events
		WHERE ad_id = $1 AND event_type = 'click'
		GROUP BY COALESCE(user_context->>'device', 'Nieznany')
		ORDER BY total DESC
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk urządzeń", http.StatusInternalServerError)
		return
	}

	result.ByInterest, queryErr = getClickBreakdown(
		h.DB,
		`
		SELECT
			i.value AS label,
			COUNT(*) AS total
		FROM events e
		CROSS JOIN LATERAL jsonb_array_elements_text(
			CASE
				WHEN jsonb_typeof(e.user_context->'interests') = 'array'
				THEN e.user_context->'interests'
				ELSE '[]'::jsonb
			END
		) AS i(value)
		WHERE e.ad_id = $1 AND e.event_type = 'click'
		GROUP BY i.value
		ORDER BY total DESC
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk zainteresowań", http.StatusInternalServerError)
		return
	}

	rows, err := h.DB.QueryContext(
		r.Context(),
		`
		SELECT
			id::text,
			created_at,
			COALESCE(user_context->>'age_group', 'Nieznany'),
			COALESCE(user_context->>'device', 'Nieznany'),
			CASE
				WHEN jsonb_typeof(user_context->'interests') = 'array'
				THEN user_context->'interests'
				ELSE '[]'::jsonb
			END
		FROM events
		WHERE ad_id = $1 AND event_type = 'click'
		ORDER BY created_at DESC
		LIMIT 100
		`,
		adID,
	)
	if err != nil {
		http.Error(w, "Błąd pobierania historii kliknięć", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var click RecentClick
		var interestsJSON []byte

		err := rows.Scan(
			&click.ID,
			&click.CreatedAt,
			&click.AgeGroup,
			&click.Device,
			&interestsJSON,
		)
		if err != nil {
			http.Error(w, "Błąd odczytu historii kliknięć", http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal(interestsJSON, &click.Interests); err != nil {
			http.Error(w, "Błąd odczytu zainteresowań", http.StatusInternalServerError)
			return
		}

		if click.Interests == nil {
			click.Interests = []string{}
		}

		result.RecentClicks = append(result.RecentClicks, click)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Błąd odczytu historii kliknięć", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func getClickBreakdown(
	db *sql.DB,
	query string,
	adID string,
) ([]ClickBreakdown, error) {
	rows, err := db.Query(query, adID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ClickBreakdown{}

	for rows.Next() {
		var item ClickBreakdown

		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
