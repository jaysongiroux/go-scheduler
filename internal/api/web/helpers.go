package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode response: %v", err)
	}
}

func RespondError(w http.ResponseWriter, status int, message string, details ...string) {
	resp := map[string]interface{}{
		"error": message,
	}
	if len(details) > 0 {
		resp["details"] = strings.Join(details, "\n")
	} else {
		resp["details"] = message
	}
	RespondJSON(w, status, resp)
}

func ResponseSuccess(w http.ResponseWriter) {
	success_json := json.RawMessage(`{"success": true}`)
	RespondJSON(w, http.StatusOK, success_json)
}

func ResponsePagedResults(w http.ResponseWriter, data any, count int, limit any, offset any) {
	response := map[string]any{
		"data":  data,
		"count": count,
	}
	if limit != nil {
		response["limit"] = limit
	}
	if offset != nil {
		response["offset"] = offset
	}
	RespondJSON(w, http.StatusOK, response)
}
