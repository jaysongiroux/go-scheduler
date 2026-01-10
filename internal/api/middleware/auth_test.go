package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	validKey := "my-super-secret-key"
	hash := sha256.Sum256([]byte(validKey))
	validKeyHash := hex.EncodeToString(hash[:])

	tests := []struct {
		name           string
		apiKeyHeader   string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Missing API Key Header",
			apiKeyHeader:   "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"missing api-key header"}`,
		},
		{
			name:           "Invalid API Key",
			apiKeyHeader:   "wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"invalid api-key"}`,
		},
		{
			name:           "Valid API Key",
			apiKeyHeader:   validKey,
			expectedStatus: http.StatusOK,
			expectedBody:   "Success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			if tt.apiKeyHeader != "" {
				req.Header.Set("api-key", tt.apiKeyHeader)
			}

			rr := httptest.NewRecorder()

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success"))
			})

			mw := AuthMiddleware(validKeyHash, nil)
			mw(nextHandler).ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			actualBody := strings.TrimSpace(rr.Body.String())
			if actualBody != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					actualBody, tt.expectedBody)
			}
		})
	}
}
