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
		// TODO: generate a secure secret
		wh.Secret = uuid.New().String()
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
	err, limitInt, offsetInt := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset", err.Error())
		return
	}

	webhookEndpoints, err := h.WebhookRepo.GetWebhookEndpoints(r.Context(), offsetInt, limitInt)
	if err != nil {
		logger.Error("Failed to get webhook endpoints: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get webhook endpoints", err.Error())
		return
	}

	web.ResponsePagedResults(w, webhookEndpoints, len(webhookEndpoints), limitInt, offsetInt)
}

func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookUID, err := uuid.Parse(r.PathValue("webhook_uid"))
	if err != nil {
		logger.Error("Failed to parse webhook UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid webhook UID", err.Error())
		return
	}

	var wh webhook.Webhook
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		logger.Error("Failed to decode request body: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if wh.Secret != "" {
		web.RespondError(w, http.StatusBadRequest, "Cannot update secret")
		return
	}

	wh.WebhookUID = webhookUID

	// check if webhook exists
	existingWh, err := h.WebhookRepo.GetWebhook(r.Context(), webhookUID)
	if err != nil {
		logger.Error("Failed to get webhook: %v", err)
		web.RespondError(w, http.StatusNotFound, "Webhook not found", err.Error())
		return
	}

	// handle if event types is empty by setting it to an empty array
	if wh.EventTypes == nil {
		wh.EventTypes = []string{}
	}

	if wh.RetryCount < 0 {
		logger.Info("Setting retry count to %v", h.Config.WebhookMaxRetries)
		wh.RetryCount = h.Config.WebhookMaxRetries
	}
	if wh.TimeoutSeconds < 0 {
		logger.Info("Setting timeout seconds to %v", h.Config.WebhookTimeoutSeconds)
		wh.TimeoutSeconds = h.Config.WebhookTimeoutSeconds
	}

	wh.UpdatedTs = time.Now().UTC().Unix()
	wh.CreatedTs = existingWh.CreatedTs
	wh.FailureCount = existingWh.FailureCount

	if err := h.WebhookRepo.UpdateWebhook(r.Context(), &wh); err != nil {
		logger.Error("Failed to update webhook: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to update webhook", err.Error())
		return
	}

	updatedwh, err := h.WebhookRepo.GetWebhook(r.Context(), webhookUID)
	if err != nil {
		logger.Error("Failed to get updated webhook: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get updated webhook", err.Error())
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

	err, limitInt, offsetInt := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset", err.Error())
		return
	}

	deliveries, err := h.WebhookRepo.GetWebhookDeliveries(r.Context(), webhookUID, limitInt, offsetInt)
	if err != nil {
		logger.Error("Failed to get webhook deliveries: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get webhook deliveries", err.Error())
		return
	}

	web.ResponsePagedResults(w, deliveries, len(deliveries), limitInt, offsetInt)
}
