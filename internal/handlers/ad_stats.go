package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type ClickBreakdown struct {
	Label       string `json:"label"`
	Impressions int    `json:"impressions"`
	Clicks      int    `json:"clicks"`
}

type RecentClick struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	AgeGroup  string    `json:"age_group"`
	Device    string    `json:"device"`
	Interests []string  `json:"interests"`
}

type AdClickStatsResponse struct {
	TotalImpressions int              `json:"total_impressions"`
	TotalClicks      int              `json:"total_clicks"`
	ByAgeGroup       []ClickBreakdown `json:"by_age_group"`
	ByDevice         []ClickBreakdown `json:"by_device"`
	ByInterest       []ClickBreakdown `json:"by_interest"`
	RecentClicks     []RecentClick    `json:"recent_clicks"`
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

	// Łączna liczba wyświetleń i kliknięć.
	err = h.DB.QueryRowContext(
		r.Context(),
		`
		SELECT
			COUNT(*) FILTER (WHERE event_type = 'impression'),
			COUNT(*) FILTER (WHERE event_type = 'click')
		FROM events
		WHERE ad_id = $1
		`,
		adID,
	).Scan(&result.TotalImpressions, &result.TotalClicks)
	if err != nil {
		http.Error(w, "Błąd pobierania podsumowania statystyk", http.StatusInternalServerError)
		return
	}

	var queryErr error

	// Statystyki według wieku.
	result.ByAgeGroup, queryErr = getClickBreakdown(
		h.DB,
		`
		SELECT
			label,
			COUNT(*) FILTER (
				WHERE event_type = 'impression'
			) AS impressions,
			COUNT(*) FILTER (
				WHERE event_type = 'click'
			) AS clicks
		FROM (
			SELECT
				COALESCE(
					NULLIF(user_context->>'age_group', ''),
					NULLIF(user_context->>'age', ''),
					NULLIF(user_context->>'target_age', ''),
					'Nieznany'
				) AS label,
				event_type
			FROM events
			WHERE ad_id = $1
		) AS age_events
		GROUP BY label
		ORDER BY clicks DESC, label
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk wieku", http.StatusInternalServerError)
		return
	}

	// Statystyki według urządzenia.
	result.ByDevice, queryErr = getClickBreakdown(
		h.DB,
		`
		SELECT
			label,
			COUNT(*) FILTER (
				WHERE event_type = 'impression'
			) AS impressions,
			COUNT(*) FILTER (
				WHERE event_type = 'click'
			) AS clicks
		FROM (
			SELECT
				COALESCE(
					NULLIF(user_context->>'device', ''),
					'Nieznany'
				) AS label,
				event_type
			FROM events
			WHERE ad_id = $1
		) AS device_events
		GROUP BY label
		ORDER BY clicks DESC, label
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk urządzeń", http.StatusInternalServerError)
		return
	}

	// Statystyki według zainteresowań.
	result.ByInterest, queryErr = getClickBreakdown(
		h.DB,
		`
		WITH interest_events AS (
			SELECT
				e.event_type,
				user_interest.value AS label
			FROM events e
			CROSS JOIN LATERAL (
				SELECT DISTINCT item.value
				FROM jsonb_array_elements_text(
					CASE
						WHEN jsonb_typeof(e.user_context->'interests') = 'array'
						THEN e.user_context->'interests'
						ELSE '[]'::jsonb
					END
				) AS item(value)
			) AS user_interest
			WHERE e.ad_id = $1
		)
		SELECT
			label,
			COUNT(*) FILTER (
				WHERE event_type = 'impression'
			) AS impressions,
			COUNT(*) FILTER (
				WHERE event_type = 'click'
			) AS clicks
		FROM interest_events
		GROUP BY label
		ORDER BY clicks DESC, label
		`,
		adID,
	)

	if queryErr != nil {
		http.Error(w, "Błąd statystyk zainteresowań", http.StatusInternalServerError)
		return
	}

	// Historia ostatnich kliknięć.
	rows, err := h.DB.QueryContext(
		r.Context(),
		`
		SELECT
			id::text,
			created_at,
			COALESCE(
				NULLIF(user_context->>'age_group', ''),
				NULLIF(user_context->>'age', ''),
				NULLIF(user_context->>'target_age', ''),
				'Nieznany'
			),
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

	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
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

		if err := rows.Scan(
			&item.Label,
			&item.Impressions,
			&item.Clicks,
		); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
