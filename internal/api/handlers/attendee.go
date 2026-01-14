package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/attendee"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// CreateAttendee handles POST /api/v1/events/{event_uid}/attendees
func (h *Handler) CreateAttendee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	var req struct {
		AccountID string          `json:"account_id"`
		Role      string          `json:"role"`
		Scope     string          `json:"scope"`
		Metadata  json.RawMessage `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.AccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	if req.Role == "" {
		req.Role = attendee.RoleAttendee
	}

	// Validate role
	if req.Role != attendee.RoleOrganizer && req.Role != attendee.RoleAttendee {
		web.RespondError(w, http.StatusBadRequest, "role must be 'organizer' or 'attendee'")
		return
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(ctx, eventUID, nil)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found")
		return
	}

	isRecurring := evt.MasterEventUID != nil || (evt.Recurrence != nil && evt.Recurrence.Rule != "")

	// Validate scope for recurring events
	if isRecurring && req.Scope == "" {
		web.RespondError(w, http.StatusBadRequest, "scope is required for recurring events")
		return
	}

	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "scope must be 'single' or 'all'")
		return
	}

	// Default scope for non-recurring events
	if !isRecurring {
		req.Scope = "single"
	}

	if req.Metadata == nil {
		req.Metadata = json.RawMessage("{}")
	}

	// Create attendee based on scope
	if req.Scope == "single" {
		newAttendee, err := h.AttendeeRepo.CreateSingleAttendee(
			ctx,
			eventUID,
			req.AccountID,
			req.Role,
			req.Metadata,
		)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to create attendee: %v", err),
			)
			return
		}

		// Queue webhook
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeCreated, map[string]interface{}{
			"attendee":  newAttendee,
			"event_uid": eventUID,
			"scope":     req.Scope,
			"count":     1,
		}, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee creation: %v", err)
		}

		web.RespondJSON(w, http.StatusCreated, map[string]interface{}{
			"attendee": newAttendee,
			"scope":    req.Scope,
			"count":    1,
		})
		return
	}

	// scope="all"
	groupID, count, err := h.AttendeeRepo.CreateSeriesAttendee(
		ctx,
		eventUID,
		req.AccountID,
		req.Role,
		req.Metadata,
	)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to create series attendees: %v", err),
		)
		return
	}

	// Queue webhook (batch if count > 100)
	webhookData := map[string]interface{}{
		"account_id":        req.AccountID,
		"event_uid":         eventUID,
		"scope":             req.Scope,
		"count":             count,
		"attendee_group_id": groupID,
	}
	if count > 100 {
		batchData := []interface{}{webhookData}
		if err := h.WebhookDispatcher.QueueBatchDelivery(ctx, workers.AttendeeCreated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for attendee creation: %v", err)
		}
	} else {
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeCreated, webhookData, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee creation: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"attendee_group_id": groupID,
		"scope":             req.Scope,
		"count":             count,
	})
}

// GetEventAttendees handles GET /api/v1/events/{event_uid}/attendees
func (h *Handler) GetEventAttendees(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	// Get optional query parameters
	roleFilter := r.URL.Query().Get("role")
	rsvpFilter := r.URL.Query().Get("rsvp_status")

	attendees, err := h.AttendeeRepo.GetEventAttendees(ctx, eventUID, roleFilter, rsvpFilter)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to get attendees: %v", err),
		)
		return
	}

	web.RespondJSON(w, http.StatusOK, attendees)
}

// GetAttendee handles GET /api/v1/events/{event_uid}/attendees/{account_id}
func (h *Handler) GetAttendee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	att, err := h.AttendeeRepo.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Attendee not found")
		return
	}

	web.RespondJSON(w, http.StatusOK, att)
}

// UpdateAttendee handles PATCH /api/v1/events/{event_uid}/attendees/{account_id}
func (h *Handler) UpdateAttendee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	var req struct {
		Role     string          `json:"role"`
		Metadata json.RawMessage `json:"metadata"`
		Scope    string          `json:"scope"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate role if provided
	if req.Role != "" && req.Role != attendee.RoleOrganizer && req.Role != attendee.RoleAttendee {
		web.RespondError(w, http.StatusBadRequest, "role must be 'organizer' or 'attendee'")
		return
	}

	// Get current attendee
	current, err := h.AttendeeRepo.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Attendee not found")
		return
	}

	// Use current values if not provided
	if req.Role == "" {
		req.Role = current.Role
	}
	if req.Metadata == nil {
		req.Metadata = current.Metadata
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(ctx, eventUID, nil)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found")
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(ctx, evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot update attendees on read-only calendar",
		)
		return
	}

	isRecurring := evt.MasterEventUID != nil || (evt.Recurrence != nil && evt.Recurrence.Rule != "")

	// Validate scope for recurring events
	if isRecurring && req.Scope == "" {
		web.RespondError(w, http.StatusBadRequest, "scope is required for recurring events")
		return
	}

	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "scope must be 'single' or 'all'")
		return
	}

	// Default scope for non-recurring events
	if !isRecurring {
		req.Scope = "single"
	}

	// Update attendee based on scope
	if req.Scope == "single" {
		updated, err := h.AttendeeRepo.UpdateSingleAttendee(
			ctx,
			eventUID,
			accountID,
			req.Role,
			req.Metadata,
		)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to update attendee: %v", err),
			)
			return
		}

		// Queue webhook
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeUpdated, map[string]interface{}{
			"attendee":  updated,
			"event_uid": eventUID,
			"scope":     req.Scope,
			"count":     1,
			"changes":   []string{"role", "metadata"},
		}, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee update: %v", err)
		}

		web.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"attendee": updated,
			"scope":    req.Scope,
			"count":    1,
		})
		return
	}

	// scope="all"
	count, err := h.AttendeeRepo.UpdateSeriesAttendee(
		ctx,
		eventUID,
		accountID,
		req.Role,
		req.Metadata,
	)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to update series attendees: %v", err),
		)
		return
	}

	// Queue webhook (batch if count > 100)
	webhookData := map[string]interface{}{
		"account_id": accountID,
		"event_uid":  eventUID,
		"scope":      req.Scope,
		"count":      count,
		"changes":    []string{"role", "metadata"},
	}
	if count > 100 {
		batchData := []interface{}{webhookData}
		if err := h.WebhookDispatcher.QueueBatchDelivery(ctx, workers.AttendeeUpdated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for attendee update: %v", err)
		}
	} else {
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeUpdated, webhookData, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee update: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"scope": req.Scope,
		"count": count,
	})
}

// DeleteAttendee handles DELETE /api/v1/events/{event_uid}/attendees/{account_id}
func (h *Handler) DeleteAttendee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	scope := r.URL.Query().Get("scope")

	// Confirm attendee exists
	_, err = h.AttendeeRepo.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Attendee not found")
		return
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(ctx, eventUID, nil)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found")
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(ctx, evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot delete attendees from read-only calendar",
		)
		return
	}

	isRecurring := evt.MasterEventUID != nil || (evt.Recurrence != nil && evt.Recurrence.Rule != "")

	// Validate scope for recurring events
	if isRecurring && scope == "" {
		web.RespondError(w, http.StatusBadRequest, "scope is required for recurring events")
		return
	}

	if scope != "" && scope != "single" && scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "scope must be 'single' or 'all'")
		return
	}

	// Default scope for non-recurring events
	if !isRecurring {
		scope = "single"
	}

	// Delete attendee based on scope
	if scope == "single" {
		remindersDeleted, err := h.AttendeeRepo.DeleteSingleAttendee(ctx, eventUID, accountID)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to delete attendee: %v", err),
			)
			return
		}

		// Queue webhook
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeDeleted, map[string]interface{}{
			"account_id":        accountID,
			"event_uid":         eventUID,
			"scope":             scope,
			"count":             1,
			"reminders_deleted": remindersDeleted,
		}, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee deletion: %v", err)
		}

		web.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message":           "Attendee removed successfully",
			"scope":             scope,
			"count":             1,
			"reminders_deleted": remindersDeleted,
		})
		return
	}

	// scope="all"
	attendeeCount, remindersDeleted, err := h.AttendeeRepo.DeleteSeriesAttendee(
		ctx,
		eventUID,
		accountID,
	)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to delete series attendees: %v", err),
		)
		return
	}

	// Queue webhook (batch if count > 100)
	webhookData := map[string]interface{}{
		"account_id":        accountID,
		"event_uid":         eventUID,
		"scope":             scope,
		"count":             attendeeCount,
		"reminders_deleted": remindersDeleted,
	}
	if attendeeCount > 100 {
		batchData := []interface{}{webhookData}
		if err := h.WebhookDispatcher.QueueBatchDelivery(ctx, workers.AttendeeDeleted, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for attendee deletion: %v", err)
		}
	} else {
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeDeleted, webhookData, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee deletion: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"message":           "Attendee removed successfully",
		"scope":             scope,
		"count":             attendeeCount,
		"reminders_deleted": remindersDeleted,
	})
}

// UpdateAttendeeRSVP handles PUT /api/v1/events/{event_uid}/attendees/{account_id}/rsvp
func (h *Handler) UpdateAttendeeRSVP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	var req struct {
		RSVPStatus string `json:"rsvp_status"`
		Scope      string `json:"scope"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate rsvp_status
	if req.RSVPStatus == "" {
		web.RespondError(w, http.StatusBadRequest, "rsvp_status is required")
		return
	}

	validRSVP := map[string]bool{
		attendee.RSVPPending:   true,
		attendee.RSVPAccepted:  true,
		attendee.RSVPDeclined:  true,
		attendee.RSVPTentative: true,
	}
	if !validRSVP[req.RSVPStatus] {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"rsvp_status must be 'pending', 'accepted', 'declined', or 'tentative'",
		)
		return
	}

	// Confirm attendee exists
	_, err = h.AttendeeRepo.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Attendee not found")
		return
	}

	// Get event to check if it's recurring
	evt, err := h.EventRepo.GetEvent(ctx, eventUID, nil)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found")
		return
	}

	isRecurring := evt.MasterEventUID != nil || (evt.Recurrence != nil && evt.Recurrence.Rule != "")

	// Validate scope for recurring events
	if isRecurring && req.Scope == "" {
		web.RespondError(w, http.StatusBadRequest, "scope is required for recurring events")
		return
	}

	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "scope must be 'single' or 'all'")
		return
	}

	// Default scope for non-recurring events
	if !isRecurring {
		req.Scope = "single"
	}

	// Update RSVP based on scope
	if req.Scope == "single" {
		updated, err := h.AttendeeRepo.UpdateSingleAttendeeRSVP(
			ctx,
			eventUID,
			accountID,
			req.RSVPStatus,
		)
		if err != nil {
			if err.Error() == "cannot update RSVP for events that have already ended" {
				web.RespondError(w, http.StatusBadRequest, err.Error())
				return
			}
			web.RespondError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to update RSVP: %v", err),
			)
			return
		}

		// Queue webhook
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeUpdated, map[string]interface{}{
			"attendee":  updated,
			"event_uid": eventUID,
			"scope":     req.Scope,
			"count":     1,
			"changes":   []string{"rsvp_status"},
		}, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee RSVP update: %v", err)
		}

		web.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"attendee": updated,
			"scope":    req.Scope,
			"count":    1,
		})
		return
	}

	// scope="all"
	count, err := h.AttendeeRepo.UpdateSeriesAttendeeRSVP(ctx, eventUID, accountID, req.RSVPStatus)
	if err != nil {
		if err.Error() == "cannot update RSVP for events that have already ended" {
			web.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to update series RSVP: %v", err),
		)
		return
	}

	// Queue webhook (batch if count > 100)
	webhookData := map[string]interface{}{
		"account_id": accountID,
		"event_uid":  eventUID,
		"scope":      req.Scope,
		"count":      count,
		"changes":    []string{"rsvp_status"},
	}
	if count > 100 {
		batchData := []interface{}{webhookData}
		if err := h.WebhookDispatcher.QueueBatchDelivery(ctx, workers.AttendeeUpdated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for attendee RSVP update: %v", err)
		}
	} else {
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.AttendeeUpdated, webhookData, nil); err != nil {
			logger.Error("Failed to queue webhook for attendee RSVP update: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"scope": req.Scope,
		"count": count,
	})
}

// TransferEventOwnership handles POST /api/v1/events/{event_uid}/transfer-ownership
func (h *Handler) TransferEventOwnership(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventUIDStr := r.PathValue("event_uid")
	eventUID, err := uuid.Parse(eventUIDStr)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event_uid")
		return
	}

	var req struct {
		NewOrganizerAccountID   string `json:"new_organizer_account_id"`
		NewOrganizerCalendarUID string `json:"new_organizer_calendar_uid"`
		Scope                   string `json:"scope"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.NewOrganizerAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "new_organizer_account_id is required")
		return
	}
	if req.NewOrganizerCalendarUID == "" {
		web.RespondError(w, http.StatusBadRequest, "new_organizer_calendar_uid is required")
		return
	}

	newCalendarUID, err := uuid.Parse(req.NewOrganizerCalendarUID)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid new_organizer_calendar_uid")
		return
	}

	// Validate new calendar exists and belongs to new organizer
	newCalendar, err := h.CalendarRepo.GetCalendar(ctx, newCalendarUID)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "New calendar not found")
		return
	}
	if newCalendar.AccountID != req.NewOrganizerAccountID {
		web.RespondError(w, http.StatusBadRequest, "Calendar does not belong to new organizer")
		return
	}

	// Get event to check ownership and if it's recurring
	evt, err := h.EventRepo.GetEvent(ctx, eventUID, nil)
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found")
		return
	}

	currentOrganizerAccountID := evt.AccountID

	isRecurring := evt.MasterEventUID != nil || (evt.Recurrence != nil && evt.Recurrence.Rule != "")

	// Validate scope for recurring events
	if isRecurring && req.Scope == "" {
		web.RespondError(w, http.StatusBadRequest, "scope is required for recurring events")
		return
	}

	if req.Scope != "" && req.Scope != "single" && req.Scope != "all" {
		web.RespondError(w, http.StatusBadRequest, "scope must be 'single' or 'all'")
		return
	}

	// Default scope for non-recurring events
	if !isRecurring {
		req.Scope = "single"
	}

	// Transfer ownership based on scope
	if req.Scope == "single" {
		err := h.AttendeeRepo.TransferOwnershipSingle(
			ctx,
			eventUID,
			currentOrganizerAccountID,
			req.NewOrganizerAccountID,
			newCalendarUID,
		)
		if err != nil {
			web.RespondError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to transfer ownership: %v", err),
			)
			return
		}

		// Queue webhook (event.updated)
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.EventUpdated, map[string]interface{}{
			"event_uid": eventUID,
			"new_organizer": map[string]string{
				"account_id":   req.NewOrganizerAccountID,
				"calendar_uid": req.NewOrganizerCalendarUID,
			},
			"scope":  req.Scope,
			"count":  1,
			"change": "ownership_transferred",
		}, nil); err != nil {
			logger.Error("Failed to queue webhook for ownership transfer: %v", err)
		}

		web.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Ownership transferred successfully",
			"new_organizer": map[string]string{
				"account_id":   req.NewOrganizerAccountID,
				"calendar_uid": req.NewOrganizerCalendarUID,
			},
			"scope": req.Scope,
			"count": 1,
		})
		return
	}

	// scope="all"
	count, err := h.AttendeeRepo.TransferOwnershipSeries(
		ctx,
		eventUID,
		currentOrganizerAccountID,
		req.NewOrganizerAccountID,
		newCalendarUID,
	)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to transfer series ownership: %v", err),
		)
		return
	}

	// Queue webhook (batch if count > 100)
	webhookData := map[string]interface{}{
		"event_uid": eventUID,
		"new_organizer": map[string]string{
			"account_id":   req.NewOrganizerAccountID,
			"calendar_uid": req.NewOrganizerCalendarUID,
		},
		"scope":  req.Scope,
		"count":  count,
		"change": "ownership_transferred",
	}

	if count > 100 {
		batchData := []interface{}{webhookData}
		if err := h.WebhookDispatcher.QueueBatchDelivery(ctx, workers.EventUpdated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for ownership transfer: %v", err)
		}
	} else {
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.EventUpdated, webhookData, nil); err != nil {
			logger.Error("Failed to queue webhook for ownership transfer: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Ownership transferred successfully",
		"new_organizer": map[string]string{
			"account_id":   req.NewOrganizerAccountID,
			"calendar_uid": req.NewOrganizerCalendarUID,
		},
		"scope": req.Scope,
		"count": count,
	})
}

// GetAttendeeEvents handles GET /api/v1/attendees/events
func (h *Handler) GetAttendeeEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters
	accountID := r.URL.Query().Get("account_id")
	startTsStr := r.URL.Query().Get("start_ts")
	endTsStr := r.URL.Query().Get("end_ts")
	roleFilter := r.URL.Query().Get("role")
	rsvpFilter := r.URL.Query().Get("rsvp_status")

	// Validate required parameters
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}
	if startTsStr == "" {
		web.RespondError(w, http.StatusBadRequest, "start_ts is required")
		return
	}
	if endTsStr == "" {
		web.RespondError(w, http.StatusBadRequest, "end_ts is required")
		return
	}

	startTs, err := strconv.ParseInt(startTsStr, 10, 64)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid start_ts")
		return
	}

	endTs, err := strconv.ParseInt(endTsStr, 10, 64)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid end_ts")
		return
	}

	// By default, exclude organizer events (show only events user is invited to)
	if roleFilter == "" {
		roleFilter = attendee.RoleAttendee
	}

	events, err := h.AttendeeRepo.GetAccountEvents(
		ctx,
		accountID,
		startTs,
		endTs,
		roleFilter,
		rsvpFilter,
	)
	if err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("Failed to get events: %v", err),
		)
		return
	}

	// Transform to response format
	type EventResponse struct {
		Event struct {
			EventUID       uuid.UUID       `json:"event_uid"`
			CalendarUID    uuid.UUID       `json:"calendar_uid"`
			StartTs        int64           `json:"start_ts"`
			EndTs          int64           `json:"end_ts"`
			Metadata       json.RawMessage `json:"metadata"`
			IsCancelled    bool            `json:"is_cancelled"`
			MasterEventUID *uuid.UUID      `json:"master_event_uid,omitempty"`
		} `json:"event"`
		Attendee struct {
			AttendeeUID uuid.UUID `json:"attendee_uid"`
			Role        string    `json:"role"`
			RSVPStatus  string    `json:"rsvp_status"`
		} `json:"attendee"`
	}

	response := make([]EventResponse, 0, len(events))
	for _, e := range events {
		var er EventResponse
		er.Event.EventUID = e.EventUID
		er.Event.CalendarUID = e.CalendarUID
		er.Event.StartTs = e.StartTs
		er.Event.EndTs = e.EndTs
		er.Event.Metadata = e.Metadata
		er.Event.IsCancelled = e.IsCancelled
		er.Event.MasterEventUID = e.MasterEventUID
		er.Attendee.AttendeeUID = e.AttendeeUID
		er.Attendee.Role = e.Role
		er.Attendee.RSVPStatus = e.RSVPStatus
		response = append(response, er)
	}

	web.RespondJSON(w, http.StatusOK, response)
}
