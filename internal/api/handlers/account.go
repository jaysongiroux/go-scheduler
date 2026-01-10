package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/account"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var acc account.Account
	if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if acc.AccountID == "" {
		acc.AccountID = uuid.New().String()
	}

	exists, err := h.AccountRepo.CheckAccountExists(r.Context(), acc.AccountID)
	if err != nil {
		logger.Error("Failed to check account exists: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to check account exists", err.Error())
		return
	}
	if exists {
		web.RespondError(w, http.StatusConflict, "Account already exists")
		return
	}

	acc.CreatedTs = time.Now().UTC().Unix()
	acc.UpdatedTs = time.Now().UTC().Unix()

	settings, err := json.Marshal(acc.Settings)
	if err != nil {
		logger.Error("Failed to marshal settings: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to create account: invalid settings",
			err.Error(),
		)
		return
	}
	metadata, err := json.Marshal(acc.Metadata)
	if err != nil {
		logger.Error("Failed to marshal metadata: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to create account: invalid metadata",
			err.Error(),
		)
		return
	}

	if len(settings) == 0 || string(settings) == "null" {
		settings = json.RawMessage("{}")
	}
	if len(metadata) == 0 || string(metadata) == "null" {
		metadata = json.RawMessage("{}")
	}

	acc.Settings = settings
	acc.Metadata = metadata

	if err := h.AccountRepo.CreateAccount(r.Context(), &acc); err != nil {
		logger.Error("Failed to create account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to create account", err.Error())
		return
	}

	fetchedAcc, err := h.AccountRepo.GetAccountByID(r.Context(), acc.AccountID)
	if err != nil {
		logger.Error("Failed to get account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get account", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusCreated, fetchedAcc)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.AccountCreated,
		fetchedAcc,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for account creation: %v", err)
	}
}

func (h *Handler) GetAccountByID(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")

	acc, err := h.AccountRepo.GetAccountByID(r.Context(), accountID)
	if err != nil {
		logger.Error("Failed to get account: %v", err)
		web.RespondError(w, http.StatusNotFound, "Account not found", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, acc)
}

func (h *Handler) DeleteAccountByID(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")

	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "Account ID is required")
		return
	}

	exists, err := h.AccountRepo.CheckAccountExists(r.Context(), accountID)
	if err != nil {
		logger.Error("Failed to check account exists: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to check account exists", err.Error())
		return
	}
	if !exists {
		web.RespondError(w, http.StatusNotFound, "Account not found")
		return
	}

	if err := h.AccountRepo.DeleteAccountByID(r.Context(), accountID); err != nil {
		logger.Error("Failed to delete account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to delete account", err.Error())
		return
	}

	web.ResponseSuccess(w)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.AccountDeleted,
		map[string]interface{}{
			"account_id": accountID,
		},
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for account deletion: %v", err)
	}
}

func (h *Handler) UpdateAccountByID(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")

	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "Account ID is required")
		return
	}

	exists, err := h.AccountRepo.CheckAccountExists(r.Context(), accountID)
	if err != nil {
		logger.Error("Failed to check account exists: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to check account exists", err.Error())
		return
	}
	if !exists {
		web.RespondError(w, http.StatusNotFound, "Account not found")
		return
	}

	var acc account.Account
	if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// if settings or metadata is provided, marshal it
	if acc.Settings != nil {
		settings, err := json.Marshal(acc.Settings)
		if err != nil {
			logger.Error("Failed to marshal settings: %v", err)
			web.RespondError(w, http.StatusInternalServerError, "Failed to update account: invalid settings", err.Error())
			return
		}
		acc.Settings = settings
	} else {
		// settings is null
		acc.Settings = json.RawMessage("{}")
	}
	if acc.Metadata != nil {
		metadata, err := json.Marshal(acc.Metadata)
		if err != nil {
			logger.Error("Failed to marshal metadata: %v", err)
			web.RespondError(w, http.StatusInternalServerError, "Failed to update account: invalid metadata", err.Error())
			return
		}
		acc.Metadata = metadata
	} else {
		// metadata is null
		acc.Metadata = json.RawMessage("{}")
	}
	if err := h.AccountRepo.UpdateAccountByID(r.Context(), accountID, &acc); err != nil {
		logger.Error("Failed to update account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to update account", err.Error())
		return
	}

	updatedAcc, err := h.AccountRepo.GetAccountByID(r.Context(), accountID)
	if err != nil {
		logger.Error("Failed to get account: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get account", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, updatedAcc)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.AccountUpdated,
		updatedAcc,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for account update: %v", err)
	}

}

func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	err, limit, offset := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset")
		return
	}

	var filters *account.AccountFilters

	metadataFilter := r.URL.Query().Get("metadata")
	settingsFilter := r.URL.Query().Get("settings")

	if metadataFilter != "" || settingsFilter != "" {
		filters = &account.AccountFilters{
			MetadataFilters: make(map[string]any),
			SettingsFilters: make(map[string]any),
		}

		if metadataFilter != "" {
			if err := json.Unmarshal([]byte(metadataFilter), &filters.MetadataFilters); err != nil {
				logger.Warn("Failed to unmarshal metadata filter: %v", err.Error())
				web.RespondError(w, http.StatusBadRequest, "Invalid metadata filter format", err.Error())
				return
			}
		}

		if settingsFilter != "" {
			if err := json.Unmarshal([]byte(settingsFilter), &filters.SettingsFilters); err != nil {
				logger.Warn("Failed to unmarshal settings filter: %v", err.Error())
				web.RespondError(w, http.StatusBadRequest, "Invalid settings filter format", err.Error())
				return
			}
		}
	}

	accounts, err := h.AccountRepo.GetAccounts(r.Context(), limit, offset, filters)
	if err != nil {
		logger.Warn("Failed to get accounts: %v", err.Error())
		web.RespondError(w, http.StatusInternalServerError, "Failed to get accounts", err.Error())
		return
	}

	web.ResponsePagedResults(w, accounts, len(accounts), limit, offset)
}
