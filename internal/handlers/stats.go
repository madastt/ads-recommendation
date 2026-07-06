package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type StatsHandler struct {
	DB *sql.DB
}

type EndpointStat struct {
	Path  string `json:"path"`
	Calls int    `json:"calls"`
}
type APIStatsResponse struct {
	TotalRequests int            `json:"total_requests"`
	AvgLatency    int            `json:"avg_latency_ms"`
	TopEndpoints  []EndpointStat `json:"top_endpoints"`
}

func (h *StatsHandler) GetAPIStats(w http.ResponseWriter, r *http.Request) {
	var stats APIStatsResponse
	h.DB.QueryRow("SELECT COUNT(*) FROM api_logs").Scan(&stats.TotalRequests)
	h.DB.QueryRow("SELECT COALESCE(ROUND(AVG(latency_ms)), 0) FROM api_logs").Scan(&stats.AvgLatency)
	rows, err := h.DB.Query(`
		SELECT path, COUNT(*) as calls 
		FROM api_logs 
		GROUP BY path 
		ORDER BY calls DESC 
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ep EndpointStat
			rows.Scan(&ep.Path, &ep.Calls)
			stats.TopEndpoints = append(stats.TopEndpoints, ep)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
