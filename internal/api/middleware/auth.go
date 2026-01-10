package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func AuthMiddleware(apiKeyHash string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestKey := r.Header.Get("api-key")

			if requestKey == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write([]byte(`{"error":"missing api-key header"}`))
				if err != nil {
					log.Error("Failed to write response: %v", err)
				}
				return
			}

			hashedToken := sha256.Sum256([]byte(requestKey))
			hashedTokenString := hex.EncodeToString(hashedToken[:])

			if hashedTokenString != apiKeyHash {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write([]byte(`{"error":"invalid api-key"}`))
				if err != nil {
					log.Error("Failed to write response: %v", err)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
