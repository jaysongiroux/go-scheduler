package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// CreateReminderRequest represents the request body for creating a reminder
type CreateReminderRequest struct {
	OffsetSeconds int64           `json:"offset_seconds"`
	AccountID     string          `json:"account_id"`
	Metadata      json.RawMessage `json:"metadata"`
	Scope         string          `json:"scope"` // "single" or "all"
}

// UpdateReminderRequest represents the request body for updating a reminder
type UpdateReminderRequest struct {
	OffsetSeconds int64           `json:"offset_seconds"`
	Metadata      json.RawMessage `json:"metadata"`
	Scope         string          `json:"scope"` // "single" or "all"
}

// DeleteReminderRequest represents the request body for deleting a reminder
type DeleteReminderRequest struct {
	Scope string `json:"scope"` // "single" or "all"
}

// CreateReminder creates a new reminder for an event
func (h *Handler) CreateReminder(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	var req CreateReminderRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate offset_seconds (must be negative - reminder before event)
	if req.OffsetSeconds > 0 {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"offset_seconds must be negative (time before event)",
		)
		return
	}

	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage("{}")
	}

	// account ID is required
	if req.AccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	// Get event to validate it exists and check if recurring
	evt, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot create reminders on read-only calendar",
		)
		return
	}

	// Validate that the account is an attendee of this event
	if h.AttendeeRepo != nil {
		isAttendee, err := h.AttendeeRepo.IsAccountAttendee(r.Context(), eventUID, req.AccountID)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to check attendee status",
				err.Error(),
			)
			return
		}
		if !isAttendee {
			web.RespondError(
				w,
				http.StatusForbidden,
				"Only attendees can create reminders for this event",
			)
			return
		}
	}

	// Validate scope for recurring events
	if (evt.IsRecurringInstance || evt.IsMasterEvent()) && req.Scope == "" {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"Scope is required for recurring events (single or all)",
		)
		return
	}

	// Validate scope value
	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "Invalid scope value. Must be 'single' or 'all'")
		return
	}

	// Default to single for non-recurring events
	if req.Scope == "" {
		req.Scope = "single"
	}

	var reminder *event.Reminder
	var groupID uuid.UUID
	var count int

	rm := &event.Reminder{
		EventUID:      eventUID,
		AccountID:     req.AccountID,
		OffsetSeconds: req.OffsetSeconds,
		Metadata:      req.Metadata,
	}
	if req.Scope == "single" {
		// Create single reminder for this event only
		reminder, err = h.ReminderRepo.CreateSingleReminder(r.Context(), rm)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to create reminder",
				err.Error(),
			)
			return
		}
		count = 1
	} else {
		// Create series reminder for all events in the series
		groupID, count, err = h.ReminderRepo.CreateSeriesReminder(
			r.Context(),
			rm,
		)
		if err != nil {
			web.RespondError(w, http.StatusInternalServerError, "Failed to create series reminder", err.Error())
			return
		}

		// For response, create a reminder object with the group ID
		reminder = &event.Reminder{
			ReminderGroupID: &groupID,
			AccountID:       req.AccountID,
			OffsetSeconds:   req.OffsetSeconds,
			Metadata:        req.Metadata,
		}
	}

	// Queue webhook
	if req.Scope == "all" && count > 1 {
		// Use batch delivery for series reminders
		batchData := make([]interface{}, count)
		for i := 0; i < count; i++ {
			batchData[i] = map[string]interface{}{
				"reminder_group_id": groupID,
				"event_uid":         eventUID,
				"scope":             req.Scope,
				"account_id":        req.AccountID,
				"offset_seconds":    req.OffsetSeconds,
				"metadata":          req.Metadata,
			}
		}
		if err := h.WebhookDispatcher.QueueBatchDelivery(
			r.Context(),
			workers.ReminderCreated,
			batchData,
		); err != nil {
			logger.Error("Failed to queue batch webhook for reminder creation: %v", err)
		}
	} else {
		// Single reminder delivery
		webhookData := map[string]interface{}{
			"reminder":   reminder,
			"event_uid":  eventUID,
			"scope":      req.Scope,
			"account_id": req.AccountID,
		}
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.ReminderCreated,
			webhookData,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for reminder creation: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"reminder": reminder,
		"scope":    req.Scope,
		"count":    count,
	})
}

// GetEventReminders returns all reminders for an event
func (h *Handler) GetEventReminders(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	// Optional: filter by account_id from query param
	accountID := r.URL.Query().Get("account_id")
	var accountIDPtr *string
	if accountID != "" {
		accountIDPtr = &accountID
	}

	// Check if event exists
	_, err = h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	reminders, err := h.ReminderRepo.GetEventReminders(r.Context(), eventUID, accountIDPtr)
	if err != nil {
		web.RespondError(w, http.StatusInternalServerError, "Failed to get reminders", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, reminders)
}

// UpdateReminder updates a specific reminder
func (h *Handler) UpdateReminder(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	reminderUID, err := uuid.Parse(r.PathValue("reminder_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid reminder UID", err.Error())
		return
	}

	var req UpdateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate offset_seconds (must be negative)
	if req.OffsetSeconds > 0 {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"offset_seconds must be negative (time before event)",
		)
		return
	}

	if len(req.Metadata) == 0 {
		req.Metadata = json.RawMessage("{}")
	}

	// Get the reminder to verify it exists and belongs to this event
	reminder, err := h.ReminderRepo.GetReminderByUID(r.Context(), reminderUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Reminder not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get reminder", err.Error())
		return
	}

	// Verify reminder belongs to this event
	if reminder.EventUID != eventUID {
		web.RespondError(w, http.StatusBadRequest, "Reminder does not belong to this event")
		return
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot update reminders on read-only calendar",
		)
		return
	}

	// Validate scope for recurring events
	if (evt.IsRecurringInstance || evt.IsMasterEvent()) && req.Scope == "" {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"Scope is required for recurring events (single or all)",
		)
		return
	}

	// Validate scope value
	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "Invalid scope value. Must be 'single' or 'all'")
		return
	}

	// Default to single for non-recurring events
	if req.Scope == "" {
		req.Scope = "single"
	}

	var count int

	rm := &event.Reminder{
		ReminderUID:   reminderUID,
		OffsetSeconds: req.OffsetSeconds,
		Metadata:      req.Metadata,
	}
	if req.Scope == "single" {
		// Update single reminder only
		err = h.ReminderRepo.UpdateSingleReminder(
			r.Context(),
			rm,
		)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to update reminder",
				err.Error(),
			)
			return
		}
		count = 1
	} else {
		// Update series reminder (all future events)
		count, err = h.ReminderRepo.UpdateSeriesReminder(
			r.Context(),
			rm,
		)
		if err != nil {
			web.RespondError(w, http.StatusInternalServerError, "Failed to update series reminder", err.Error())
			return
		}
	}

	// Get updated reminder
	updatedReminder, err := h.ReminderRepo.GetReminderByUID(r.Context(), reminderUID)
	if err != nil {
		logger.Error("Failed to get updated reminder: %v", err)
		// Don't fail the request, just log
		updatedReminder = reminder
	}

	// Queue webhook
	if req.Scope == "all" && count > 1 {
		// Use batch delivery for series reminders
		batchData := make([]interface{}, count)
		for i := 0; i < count; i++ {
			batchData[i] = map[string]interface{}{
				"reminder_uid":   reminderUID,
				"event_uid":      eventUID,
				"scope":          req.Scope,
				"offset_seconds": req.OffsetSeconds,
				"metadata":       req.Metadata,
			}
		}
		if err := h.WebhookDispatcher.QueueBatchDelivery(
			r.Context(),
			workers.ReminderUpdated,
			batchData,
		); err != nil {
			logger.Error("Failed to queue batch webhook for reminder update: %v", err)
		}
	} else {
		// Single reminder delivery
		webhookData := map[string]interface{}{
			"reminder":        updatedReminder,
			"event_uid":       eventUID,
			"scope":           req.Scope,
			"events_affected": count,
		}
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.ReminderUpdated,
			webhookData,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for reminder update: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"reminder": updatedReminder,
		"scope":    req.Scope,
		"count":    count,
	})
}

// DeleteReminder removes a reminder (soft delete with archived flag)
func (h *Handler) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	reminderUID, err := uuid.Parse(r.PathValue("reminder_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid reminder UID", err.Error())
		return
	}

	// Get scope from query parameter
	scope := r.URL.Query().Get("scope")

	// Get the reminder to verify it exists and belongs to this event
	reminder, err := h.ReminderRepo.GetReminderByUID(r.Context(), reminderUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Reminder not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get reminder", err.Error())
		return
	}

	// Verify reminder belongs to this event
	if reminder.EventUID != eventUID {
		web.RespondError(w, http.StatusBadRequest, "Reminder does not belong to this event")
		return
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot delete reminders from read-only calendar",
		)
		return
	}

	// Validate scope for recurring events
	if (evt.IsRecurringInstance || evt.IsMasterEvent()) && scope == "" {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"Scope is required for recurring events (single or all)",
		)
		return
	}

	// Validate scope value
	if scope != "" && scope != "single" && scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "Invalid scope value. Must be 'single' or 'all'")
		return
	}

	// Default to single for non-recurring events
	if scope == "" {
		scope = "single"
	}

	var count int

	if scope == "single" {
		// Delete single reminder only
		err = h.ReminderRepo.DeleteSingleReminder(r.Context(), reminderUID)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to delete reminder",
				err.Error(),
			)
			return
		}
		count = 1
	} else {
		// Delete series reminder (all future events)
		count, err = h.ReminderRepo.DeleteSeriesReminder(r.Context(), reminderUID)
		if err != nil {
			web.RespondError(w, http.StatusInternalServerError, "Failed to delete series reminder", err.Error())
			return
		}
	}

	// Queue webhook
	if scope == "all" && count > 1 {
		// Use batch delivery for series reminders
		batchData := make([]interface{}, count)
		for i := 0; i < count; i++ {
			batchData[i] = map[string]interface{}{
				"reminder_uid": reminderUID,
				"event_uid":    eventUID,
				"scope":        scope,
			}
		}
		if err := h.WebhookDispatcher.QueueBatchDelivery(
			r.Context(),
			workers.ReminderDeleted,
			batchData,
		); err != nil {
			logger.Error("Failed to queue batch webhook for reminder deletion: %v", err)
		}
	} else {
		// Single reminder delivery
		webhookData := map[string]interface{}{
			"reminder_uid":    reminderUID,
			"event_uid":       eventUID,
			"scope":           scope,
			"events_affected": count,
		}
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.ReminderDeleted,
			webhookData,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for reminder deletion: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Reminder deleted successfully",
		"scope":   scope,
		"count":   count,
	})
}
