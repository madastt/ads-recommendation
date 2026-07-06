package middleware

import (
	"database/sql"
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
func APILogger(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(lrw, r)
			latency := int(time.Since(start).Milliseconds())
			go func(method, path string, status, lat int, ua string) {
				_, _ = db.Exec(
					`INSERT INTO api_logs (method, path, status_code, latency_ms, user_agent) 
					 VALUES ($1, $2, $3, $4, $5)`,
					method, path, status, lat, ua,
				)
			}(r.Method, r.URL.Path, lrw.statusCode, latency, r.UserAgent())
		})
	}
}
