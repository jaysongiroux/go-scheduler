package middleware

import (
	"net/http"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			log.Info("→ %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			log.Info("← %s %s [%d] took %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
		})
	}
}
