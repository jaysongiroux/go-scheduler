package middleware

import (
	"net/http"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			log.Info("→ %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			// Surface API errors in logs: 4xx as warning, 5xx as error
			if wrapped.statusCode >= 500 {
				log.Error("← %s %s [%d] took %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
			} else if wrapped.statusCode >= 400 {
				log.Warn("← %s %s [%d] took %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
			} else {
				log.Info("← %s %s [%d] took %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
			}
		})
	}
}
