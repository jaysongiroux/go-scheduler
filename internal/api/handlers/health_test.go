package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheck(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HealthCheck(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	assert.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "healthy", resp["status"])
	_, err := strconv.ParseInt(resp["time"], 10, 64)
	assert.NoError(t, err)
}
