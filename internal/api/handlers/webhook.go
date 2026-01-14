package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/webhook"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var wh webhook.Webhook
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Generate UUID and secret if not provided
	if wh.WebhookUID == uuid.Nil {
		wh.WebhookUID = uuid.New()
	}
	if wh.Secret == "" {
		secret, err := webhook.GenerateWebhookSecret()
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to generate webhook secret",
				err.Error(),
			)
			return
		}

		wh.Secret = secret
	}

	// handle if event types is empty by setting it to an empty array
	if wh.EventTypes == nil {
		wh.EventTypes = []string{}
	}

	// handle if retry count is not provided by setting it to the default configuration value
	if wh.RetryCount == 0 {
		wh.RetryCount = h.Config.WebhookMaxRetries
	}

	// handle if timeout seconds is not provided by setting it to the default configuration value
	if wh.TimeoutSeconds == 0 {
		wh.TimeoutSeconds = h.Config.WebhookTimeoutSeconds
	}

	wh.CreatedTs = time.Now().UTC().Unix()
	wh.UpdatedTs = time.Now().UTC().Unix()

	if err := h.WebhookRepo.CreateWebhook(r.Context(), &wh); err != nil {
		logger.Error("Failed to create webhook: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to create webhook", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusCreated, wh)
}

func (h *Handler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	webhookUID, err := uuid.Parse(r.PathValue("webhook_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid webhook UID", err.Error())
		return
	}

	wh, err := h.WebhookRepo.GetWebhook(r.Context(), webhookUID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Webhook not found", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, wh)
}

func (h *Handler) GetWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	limitInt, offsetInt, err := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset", err.Error())
		return
	}

	webhookEndpoints, err := h.WebhookRepo.GetWebhookEndpoints(r.Context(), offsetInt, limitInt)
	if err != nil {
		logger.Error("Failed to get webhook endpoints: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to get webhook endpoints",
			err.Error(),
		)
		return
	}

	web.ResponsePagedResults(w, webhookEndpoints, len(webhookEndpoints), limitInt, offsetInt)
}

// UpdateWebhookRequest is used to parse partial updates for webhooks
type UpdateWebhookRequest struct {
	URL            *string  `json:"url,omitempty"`
	EventTypes     []string `json:"event_types,omitempty"`
	IsActive       *bool    `json:"is_active,omitempty"`
	RetryCount     *int     `json:"retry_count,omitempty"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
	// #nosec G117 -- webhook signing secret by design; not updatable but we check for it
	Secret string `json:"secret,omitempty"`
}

func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookUID, err := uuid.Parse(r.PathValue("webhook_uid"))
	if err != nil {
		logger.Error("Failed to parse webhook UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid webhook UID", err.Error())
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request body: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Secret != "" {
		web.RespondError(w, http.StatusBadRequest, "Cannot update secret")
		return
	}

	// Get existing webhook
	existingWh, err := h.WebhookRepo.GetWebhook(r.Context(), webhookUID)
	if err != nil {
		logger.Error("Failed to get webhook: %v", err)
		web.RespondError(w, http.StatusNotFound, "Webhook not found", err.Error())
		return
	}

	// Build updated webhook by merging with existing values
	wh := webhook.Webhook{
		WebhookUID:     webhookUID,
		URL:            existingWh.URL,
		EventTypes:     existingWh.EventTypes,
		IsActive:       existingWh.IsActive,
		RetryCount:     existingWh.RetryCount,
		TimeoutSeconds: existingWh.TimeoutSeconds,
		Secret:         existingWh.Secret,
		CreatedTs:      existingWh.CreatedTs,
		FailureCount:   existingWh.FailureCount,
		UpdatedTs:      time.Now().UTC().Unix(),
	}

	// Apply updates from request
	if req.URL != nil {
		wh.URL = *req.URL
	}
	if req.EventTypes != nil {
		wh.EventTypes = req.EventTypes
	}
	if req.IsActive != nil {
		wh.IsActive = *req.IsActive
	}
	if req.RetryCount != nil {
		if *req.RetryCount < 0 {
			logger.Info("Setting retry count to %v", h.Config.WebhookMaxRetries)
			wh.RetryCount = h.Config.WebhookMaxRetries
		} else {
			wh.RetryCount = *req.RetryCount
		}
	}
	if req.TimeoutSeconds != nil {
		if *req.TimeoutSeconds <= 0 {
			logger.Info("Setting timeout seconds to %v", h.Config.WebhookTimeoutSeconds)
			wh.TimeoutSeconds = h.Config.WebhookTimeoutSeconds
		} else {
			wh.TimeoutSeconds = *req.TimeoutSeconds
		}
	}

	if err := h.WebhookRepo.UpdateWebhook(r.Context(), &wh); err != nil {
		logger.Error("Failed to update webhook: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to update webhook", err.Error())
		return
	}

	updatedwh, err := h.WebhookRepo.GetWebhook(r.Context(), webhookUID)
	if err != nil {
		logger.Error("Failed to get updated webhook: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to get updated webhook",
			err.Error(),
		)
		return
	}

	web.RespondJSON(w, http.StatusOK, updatedwh)
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	webhookUID, err := uuid.Parse(r.PathValue("webhook_uid"))
	if err != nil {
		logger.Error("Failed to parse webhook UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid webhook UID", err.Error())
		return
	}

	if err := h.WebhookRepo.DeleteWebhook(r.Context(), webhookUID); err != nil {
		logger.Error("Failed to delete webhook: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to delete webhook", err.Error())
		return
	}

	web.ResponseSuccess(w)
}

func (h *Handler) GetWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	webhookUID, err := uuid.Parse(r.PathValue("webhook_uid"))
	if err != nil {
		logger.Error("Failed to parse webhook UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid webhook UID", err.Error())
		return
	}

	limitInt, offsetInt, err := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset", err.Error())
		return
	}

	deliveries, err := h.WebhookRepo.GetWebhookDeliveries(
		r.Context(),
		webhookUID,
		limitInt,
		offsetInt,
	)
	if err != nil {
		logger.Error("Failed to get webhook deliveries: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to get webhook deliveries",
			err.Error(),
		)
		return
	}

	web.ResponsePagedResults(w, deliveries, len(deliveries), limitInt, offsetInt)
}
