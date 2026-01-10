package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handlerStatus  int
		expectedStatus int
	}{
		{
			name:           "Success Response (200)",
			handlerStatus:  http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Error Response (404)",
			handlerStatus:  http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Server Error (500)",
			handlerStatus:  http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
			rr := httptest.NewRecorder()

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.handlerStatus)
			})

			// 3. Pass the mock logger here
			mw := LoggingMiddleware(logger.New("test", logger.InfoLevel))

			mw(nextHandler).ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tt.expectedStatus)
			}
		})
	}
}
