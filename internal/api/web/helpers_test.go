package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       interface{}
		wantStatus int
		wantBody   string
	}{
		{
			name:       "simple map response",
			status:     http.StatusOK,
			data:       map[string]string{"hello": "world"},
			wantStatus: http.StatusOK,
			wantBody:   `{"hello":"world"}`,
		},
		{
			name:       "slice response",
			status:     http.StatusCreated,
			data:       []int{1, 2, 3},
			wantStatus: http.StatusCreated,
			wantBody:   `[1,2,3]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			RespondJSON(rec, tt.status, tt.data)

			res := rec.Result()
			defer res.Body.Close()

			// Status code
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.StatusCode)
			}

			// Content-Type
			ct := res.Header.Get("Content-Type")
			if ct != "application/json" {
				t.Fatalf("expected Content-Type application/json, got %s", ct)
			}

			// Body
			body := strings.TrimSpace(rec.Body.String())
			if body != tt.wantBody {
				t.Fatalf("expected body %s, got %s", tt.wantBody, body)
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		details    []string
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name:       "error without details",
			status:     http.StatusBadRequest,
			message:    "invalid request",
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]interface{}{
				"error": "invalid request",
			},
		},
		{
			name:       "error with details",
			status:     http.StatusInternalServerError,
			message:    "internal error",
			details:    []string{"db failed", "timeout"},
			wantStatus: http.StatusInternalServerError,
			wantBody: map[string]interface{}{
				"error":   "internal error",
				"details": "db failed\ntimeout",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			RespondError(rec, tt.status, tt.message, tt.details...)

			res := rec.Result()
			defer res.Body.Close()

			// Status code
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.StatusCode)
			}

			// Decode body
			var got map[string]interface{}
			if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			// Compare expected fields
			for k, v := range tt.wantBody {
				if got[k] != v {
					t.Fatalf("expected %s=%v, got %v", k, v, got[k])
				}
			}
		})
	}
}
