package handlers

import (
	"net/http"

	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// delete all entries in the database for an account
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	if err := h.AccountRepo.DeleteAccount(r.Context(), accountID); err != nil {
		logger.Error("Failed to delete account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to delete account", err.Error())
		return
	}

	web.ResponseSuccess(w)
}
