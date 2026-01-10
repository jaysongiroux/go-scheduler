package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/api/web"
)

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	web.RespondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   fmt.Sprintf("%d", time.Now().UTC().Unix()),
	})
}
