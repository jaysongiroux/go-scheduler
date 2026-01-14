package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
	_ "time/tzdata" // Embed timezone database for IANA timezone support

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/rrule"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// LocalStartLayout is the expected format for local_start (RFC3339 without timezone offset)
const LocalStartLayout = "2006-01-02T15:04:05"

// validateAndProcessTimezone validates timezone and local_start fields.
// If both are provided, it computes start_ts from them.
// Returns an error message if validation fails, empty string otherwise.
func validateAndProcessTimezone(evt *event.Event) string {
	// If timezone is provided, validate it
	if evt.Timezone != nil && *evt.Timezone != "" {
		_, err := time.LoadLocation(*evt.Timezone)
		if err != nil {
			return "Invalid timezone: must be a valid IANA timezone (e.g., America/New_York)"
		}

		// If local_start is also provided, compute start_ts from timezone + local_start
		if evt.LocalStart != nil && *evt.LocalStart != "" {
			loc, _ := time.LoadLocation(*evt.Timezone)
			localTime, err := time.ParseInLocation(LocalStartLayout, *evt.LocalStart, loc)
			if err != nil {
				return "Invalid local_start: must be in format 2006-01-02T15:04:05"
			}

			// Preserve the original duration before overriding start_ts
			originalDuration := evt.EndTs - evt.StartTs
			if originalDuration <= 0 {
				originalDuration = evt.Duration
			}

			// Convert to UTC timestamp
			evt.StartTs = localTime.UTC().Unix()

			// Recompute end_ts to preserve the original duration
			if originalDuration > 0 {
				evt.EndTs = evt.StartTs + originalDuration
			}
		}
	}
	return ""
}

// computeLocalStartFromUTC computes local_start from start_ts if timezone is set but local_start is not
func computeLocalStartFromUTC(evt *event.Event) {
	if evt.Timezone != nil && *evt.Timezone != "" &&
		(evt.LocalStart == nil || *evt.LocalStart == "") {
		loc, err := time.LoadLocation(*evt.Timezone)
		if err != nil {
			return
		}
		localTime := time.Unix(evt.StartTs, 0).In(loc)
		localStart := localTime.Format(LocalStartLayout)
		evt.LocalStart = &localStart
	}
}

// UpdateScope defines the scope of an update or delete operation for recurring events
type UpdateScope string

const (
	ScopeSingle UpdateScope = "single" // Update/delete only this instance
	ScopeFuture UpdateScope = "future" // Update/delete this and all future instances
	ScopeAll    UpdateScope = "all"    // Update/delete entire series
)

const (
	MaxEventDuration = 86400 // 24 hours in seconds
)

// checkCalendarPermission verifies that the user has the required permission for the calendar
// Returns true if the user is the owner or a confirmed member with the required role
func (h *Handler) checkCalendarPermission(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
	requiredRole string,
) (bool, error) {
	// Get the user's role for this calendar
	role, err := h.CalendarMemberRepo.GetMemberRole(ctx, accountID, calendarUID)
	if err != nil {
		return false, err
	}

	// If no role found, user is not a member or owner
	if role == "" {
		return false, nil
	}

	// If user needs write access, check they have write role
	if requiredRole == "write" && role != "write" {
		return false, nil
	}

	// User has sufficient permissions
	return true, nil
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var evt event.Event
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if evt.CalendarUID == uuid.Nil {
		web.RespondError(w, http.StatusBadRequest, "calendar_uid is required")
		return
	}

	// Validate and process timezone/local_start if provided
	// This may compute start_ts from timezone + local_start
	if errMsg := validateAndProcessTimezone(&evt); errMsg != "" {
		web.RespondError(w, http.StatusBadRequest, errMsg)
		return
	}

	// If timezone is set but local_start was not provided, compute it from start_ts
	computeLocalStartFromUTC(&evt)

	if evt.StartTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "start_ts is required and must be positive")
		return
	}

	if evt.EndTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "end_ts is required and must be positive")
		return
	}

	if evt.EndTs-evt.StartTs > MaxEventDuration {
		web.RespondError(w, http.StatusBadRequest, "Event duration cannot be more than 24 hours")
		return
	}

	if evt.EndTs <= evt.StartTs {
		web.RespondError(w, http.StatusBadRequest, "end_ts must be greater than start_ts")
		return
	}

	// Validate calendar exists and check if it's read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), evt.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	// Check if calendar is read-only (e.g., ICS import)
	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot create events on read-only calendar",
		)
		return
	}

	// Get the requesting user's account ID from request body
	requestingAccountID := evt.AccountID
	if requestingAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required in request body")
		return
	}

	// Check if user has permission to create events on this calendar
	hasPermission, err := h.checkCalendarPermission(
		r.Context(),
		requestingAccountID,
		evt.CalendarUID,
		"write",
	)
	if err != nil {
		logger.Error("Failed to check calendar permission: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to check permissions",
			err.Error(),
		)
		return
	}

	if !hasPermission {
		web.RespondError(
			w,
			http.StatusForbidden,
			"You do not have permission to create events on this calendar",
		)
		return
	}

	if evt.EventUID == uuid.Nil {
		evt.EventUID = uuid.New()
	}

	evt.CreatedTs = time.Now().UTC().Unix()
	evt.UpdatedTs = time.Now().UTC().Unix()
	evt.Duration = evt.EndTs - evt.StartTs
	if evt.Duration <= 0 {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"Invalid event duration: end_ts must be greater than start_ts",
		)
		return
	}

	if evt.Recurrence != nil && evt.Recurrence.Rule != "" {
		logger.Info("Creating recurring event with rule: %s", evt.Recurrence.Rule)

		if err := rrule.ValidateRRule(evt.Recurrence); err != nil {
			web.RespondError(w, http.StatusBadRequest, "Invalid recurrence rule", err.Error())
			return
		}

		// Create master event with pre-generated instances
		window := h.GenerationWindow()
		master, _, err := h.EventRepo.CreateEventWithInstances(r.Context(), &evt, window)
		if err != nil {
			logger.Warn("Failed to create recurring event: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to create event",
				err.Error(),
			)
			return
		}

		// Auto-create organizer attendee for master and all instances (scope="all")
		if h.AttendeeRepo != nil {
			_, attendeeCount, err := h.AttendeeRepo.CreateSeriesAttendee(
				r.Context(),
				master.EventUID,
				requestingAccountID,
				"organizer",
				json.RawMessage("{}"),
			)
			if err != nil {
				logger.Warn("Failed to create organizer attendee for recurring event: %v", err)
				// Don't fail the request, just log the warning
			} else {
				logger.Debug("Auto-created organizer attendee for %d events in recurring series", attendeeCount)
			}
		}

		web.RespondJSON(w, http.StatusCreated, master)

		// Queue webhook delivery for recurring event
		if err := h.WebhookDispatcher.QueueDelivery(
			r.Context(),
			workers.EventCreated,
			master,
			&master.StartTs,
		); err != nil {
			logger.Error("Failed to queue webhook for event creation: %v", err)
		}

		return
	}

	// check null metadata
	if len(evt.Metadata) == 0 {
		evt.Metadata = json.RawMessage("{}")
	}

	// check null exdates
	if evt.ExDatesTs == nil {
		evt.ExDatesTs = []int64{}
	}

	if evt.StartTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "Invalid start timestamp")
		return
	}

	// Non-recurring event
	createdEvent, err := h.EventRepo.CreateEvent(r.Context(), &evt)
	if err != nil {
		logger.Warn("Failed to create event: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to create event", err.Error())
		return
	}

	// Auto-create organizer attendee for single event
	if h.AttendeeRepo != nil {
		_, err := h.AttendeeRepo.CreateSingleAttendee(
			r.Context(),
			createdEvent.EventUID,
			requestingAccountID,
			"organizer",
			json.RawMessage("{}"),
		)
		if err != nil {
			logger.Warn("Failed to create organizer attendee for event: %v", err)
			// Don't fail the request, just log the warning
		}
	}

	web.RespondJSON(w, http.StatusCreated, createdEvent)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.EventCreated,
		evt,
		&evt.StartTs,
	); err != nil {
		logger.Error("Failed to queue webhook for event creation: %v", err)
	}
}

func (h *Handler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	// Read body once and parse both scope and event data
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	// Extract scope
	scopeBody := struct {
		Scope UpdateScope `json:"scope"`
	}{}
	if err := json.Unmarshal(bodyBytes, &scopeBody); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body for scope", err.Error())
		return
	}
	scope := UpdateScope(scopeBody.Scope)

	// Parse the full event data (scope field will be ignored by Event struct)
	var updateData event.Event
	if err := json.Unmarshal(bodyBytes, &updateData); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate and process timezone/local_start if provided
	if errMsg := validateAndProcessTimezone(&updateData); errMsg != "" {
		web.RespondError(w, http.StatusBadRequest, errMsg)
		return
	}

	// If timezone is set but local_start was not provided, compute it from start_ts
	computeLocalStartFromUTC(&updateData)

	if updateData.StartTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "start_ts is required and must be positive")
		return
	}

	if updateData.EndTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "end_ts is required and must be positive")
		return
	}

	if updateData.EndTs <= updateData.StartTs {
		web.RespondError(w, http.StatusBadRequest, "end_ts must be greater than start_ts")
		return
	}

	if updateData.EndTs-updateData.StartTs > MaxEventDuration {
		web.RespondError(w, http.StatusBadRequest, "Event duration cannot be more than 24 hours")
		return
	}

	// Get the existing event
	existingEvent, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), existingEvent.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot update events on read-only calendar",
		)
		return
	}

	// Get the requesting user's account ID from request body
	requestingAccountID := updateData.AccountID
	if requestingAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required in request body")
		return
	}

	// Check if user has permission to update events on this calendar
	hasPermission, err := h.checkCalendarPermission(
		r.Context(),
		requestingAccountID,
		existingEvent.CalendarUID,
		"write",
	)
	if err != nil {
		logger.Error("Failed to check calendar permission: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to check permissions",
			err.Error(),
		)
		return
	}

	if !hasPermission {
		web.RespondError(
			w,
			http.StatusForbidden,
			"You do not have permission to update events on this calendar",
		)
		return
	}

	// Preserve immutable fields from existing event
	updateData.CalendarUID = existingEvent.CalendarUID
	updateData.AccountID = existingEvent.AccountID
	updateData.EventUID = eventUID

	// handle if ex dates is empty by setting them to an empty array
	if updateData.ExDatesTs == nil {
		updateData.ExDatesTs = []int64{}
	}

	// handle if metadata is empty by setting it to an empty object
	if updateData.Metadata == nil {
		updateData.Metadata = json.RawMessage("{}")
	}

	// handle if recurrence is empty by setting it to nil
	if updateData.Recurrence == nil || rrule.IsRRuleEmpty(updateData.Recurrence) {
		updateData.Recurrence = nil
	}

	// Handle based on event type and scope
	if existingEvent.IsRecurringInstance {
		// This is an instance of a recurring event
		switch scope {
		case ScopeSingle, "":
			// Update only this instance, mark as modified
			existingEvent.StartTs = updateData.StartTs
			existingEvent.Duration = updateData.Duration
			existingEvent.EndTs = updateData.EndTs
			existingEvent.Metadata = updateData.Metadata
			existingEvent.IsModified = true

			if err := h.EventRepo.UpdateEvent(r.Context(), existingEvent); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to update instance",
					err.Error(),
				)
				return
			}
			web.RespondJSON(w, http.StatusOK, existingEvent)

		case ScopeFuture:
			// Update this and all future instances - split the series
			if existingEvent.MasterEventUID == nil {
				web.RespondError(w, http.StatusBadRequest, "Instance has no master event")
				return
			}

			// Get master event
			master, err := h.EventRepo.GetEvent(
				r.Context(),
				*existingEvent.MasterEventUID,
				&[]bool{false}[0],
			)
			if err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to get master event",
					err.Error(),
				)
				return
			}

			// Add UNTIL to old master (day before this instance)
			// This effectively ends the old series
			splitTs := existingEvent.StartTs
			inactiveStatus := event.RecurrenceStatusInactive
			master.RecurrenceStatus = &inactiveStatus
			master.RecurrenceEndTs = &splitTs
			if err := h.EventRepo.UpdateEvent(r.Context(), master); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to update master event",
					err.Error(),
				)
				return
			}

			// Delete future instances of old master
			if err := h.EventRepo.DeleteFutureInstances(r.Context(), master.EventUID, splitTs); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to delete future instances",
					err.Error(),
				)
				return
			}

			// Create new master event with updated properties
			newMaster := &event.Event{
				EventUID:    uuid.New(),
				CalendarUID: master.CalendarUID,
				AccountID:   master.AccountID,
				StartTs:     updateData.StartTs,
				Duration:    updateData.Duration,
				EndTs:       updateData.EndTs,
				Recurrence:  master.Recurrence,
				Metadata:    updateData.Metadata,
				CreatedTs:   time.Now().UTC().Unix(),
				UpdatedTs:   time.Now().UTC().Unix(),
			}

			window := h.GenerationWindow()
			newMaster, instances, err := h.EventRepo.CreateEventWithInstances(
				r.Context(),
				newMaster,
				window,
			)
			if err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to create new series",
					err.Error(),
				)
				return
			}

			web.RespondJSON(w, http.StatusOK, map[string]interface{}{
				"old_master":      master,
				"new_master":      newMaster,
				"instances_count": len(instances),
			})

		case ScopeAll:
			// Update entire series - update master and regenerate instances
			if existingEvent.MasterEventUID == nil {
				web.RespondError(w, http.StatusBadRequest, "Instance has no master event")
				return
			}

			master, err := h.EventRepo.GetEvent(
				r.Context(),
				*existingEvent.MasterEventUID,
				&[]bool{false}[0],
			)
			if err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to get master event",
					err.Error(),
				)
				return
			}

			// Update master with new values
			master.StartTs = updateData.StartTs
			master.Duration = updateData.Duration
			master.EndTs = updateData.EndTs
			master.Metadata = updateData.Metadata
			if updateData.Recurrence != nil {
				master.Recurrence = updateData.Recurrence
			}

			if err := h.EventRepo.UpdateEvent(r.Context(), master); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to update master event",
					err.Error(),
				)
				return
			}

			// Delete future instances and regenerate
			now := time.Now().UTC().Unix()
			logger.Info(
				"Deleting future instances for master event %s up to %d",
				master.EventUID,
				now,
			)
			if err := h.EventRepo.DeleteFutureInstances(r.Context(), master.EventUID, now); err != nil {
				logger.Error("Failed to delete future instances: %v", err)
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to delete future instances",
					err.Error(),
				)
				return
			}

			web.RespondJSON(w, http.StatusOK, map[string]interface{}{
				"master":  master,
				"message": "Series updated. Future instances will be regenerated by background worker.",
			})
			return
		default:
			web.RespondError(
				w,
				http.StatusBadRequest,
				"Invalid scope. Use 'single', 'future', or 'all'. Got: "+string(scope),
			)
		}
		return
	}

	// This is a master event or non-recurring event
	if existingEvent.IsMasterEvent() {
		// This is a master recurring event
		switch scope {
		case ScopeSingle, "":
			// Update only the master event itself (not instances)
			// This updates the "template" for future instances but doesn't modify existing ones
			existingEvent.StartTs = updateData.StartTs
			existingEvent.Duration = updateData.Duration
			existingEvent.EndTs = updateData.EndTs
			existingEvent.Metadata = updateData.Metadata
			if updateData.Recurrence != nil {
				existingEvent.Recurrence = updateData.Recurrence
			}

			if err := h.EventRepo.UpdateEvent(r.Context(), existingEvent); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to update master event",
					err.Error(),
				)
				return
			}

			web.RespondJSON(w, http.StatusOK, existingEvent)

			// Queue webhook delivery
			if err := h.WebhookDispatcher.QueueDelivery(
				r.Context(),
				workers.EventUpdated,
				*existingEvent,
				&existingEvent.StartTs,
			); err != nil {
				logger.Error("Failed to queue webhook for event update: %v", err)
			}
			return

		case ScopeAll:
			// Update master event with new values
			existingEvent.StartTs = updateData.StartTs
			existingEvent.Duration = updateData.Duration
			existingEvent.EndTs = updateData.EndTs
			existingEvent.Metadata = updateData.Metadata
			if updateData.Recurrence != nil {
				existingEvent.Recurrence = updateData.Recurrence
			}

			if err := h.EventRepo.UpdateEvent(r.Context(), existingEvent); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to update master event",
					err.Error(),
				)
				return
			}

			// Delete future instances and regenerate
			now := time.Now().UTC().Unix()
			logger.Info(
				"Deleting future instances for master event %s from %d",
				existingEvent.EventUID,
				now,
			)
			if err := h.EventRepo.DeleteFutureInstances(r.Context(), existingEvent.EventUID, now); err != nil {
				logger.Error("Failed to delete future instances: %v", err)
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to delete future instances",
					err.Error(),
				)
				return
			}

			web.RespondJSON(w, http.StatusOK, existingEvent)

			// Queue webhook delivery
			if err := h.WebhookDispatcher.QueueDelivery(
				r.Context(),
				workers.EventUpdated,
				*existingEvent,
				&existingEvent.StartTs,
			); err != nil {
				logger.Error("Failed to queue webhook for event update: %v", err)
			}
			return

		case ScopeFuture:
			web.RespondError(
				w,
				http.StatusBadRequest,
				"Cannot use 'future' scope on master event. Use 'all' to update the entire series.",
			)
			return

		default:
			web.RespondError(
				w,
				http.StatusBadRequest,
				"Invalid scope for master event. Use 'all'. Got: "+string(scope),
			)
			return
		}
	}

	// This is a non-recurring event
	if err := h.EventRepo.UpdateEvent(r.Context(), &updateData); err != nil {
		web.RespondError(w, http.StatusInternalServerError, "Failed to update event", err.Error())
		return
	}

	updateData.CreatedTs = existingEvent.CreatedTs
	updateData.UpdatedTs = time.Now().UTC().Unix()

	web.RespondJSON(w, http.StatusOK, updateData)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.EventUpdated,
		updateData,
		&updateData.StartTs,
	); err != nil {
		logger.Error("Failed to queue webhook for event update: %v", err)
	}
}

func (h *Handler) ToggleCancelledStatusEvent(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	if err := h.EventRepo.ToggleCancelledStatusEvent(r.Context(), eventUID); err != nil {
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to toggle cancelled status",
			err.Error(),
		)
		return
	}

	evt, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, evt)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.EventUpdated,
		evt,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for event update: %v", err)
	}
}

func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	// Get scope and account_id from query parameters (DELETE doesn't have a body)
	scope := UpdateScope(r.URL.Query().Get("scope"))
	requestingAccountID := r.URL.Query().Get("account_id")
	if requestingAccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	// Get the existing event to determine how to delete
	existingEvent, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
			return
		}
		web.RespondError(w, http.StatusInternalServerError, "Failed to get event", err.Error())
		return
	}

	// Check if calendar is read-only
	cal, err := h.CalendarRepo.GetCalendar(r.Context(), existingEvent.CalendarUID)
	if err != nil {
		logger.Warn("Failed to get calendar: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	if cal.IsReadOnly {
		web.RespondError(
			w,
			http.StatusForbidden,
			"Cannot delete events from read-only calendar",
		)
		return
	}

	// Check if user has permission to delete events on this calendar
	hasPermission, err := h.checkCalendarPermission(
		r.Context(),
		requestingAccountID,
		existingEvent.CalendarUID,
		"write",
	)
	if err != nil {
		logger.Error("Failed to check calendar permission: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to check permissions",
			err.Error(),
		)
		return
	}

	if !hasPermission {
		web.RespondError(
			w,
			http.StatusForbidden,
			"You do not have permission to delete events on this calendar",
		)
		return
	}

	if existingEvent.IsRecurringInstance {
		// This is an instance of a recurring event
		switch scope {
		case ScopeSingle:
			// Delete only this instance - add to exdates and cancel
			if existingEvent.MasterEventUID != nil && existingEvent.OriginalStartTs != nil {
				if err := h.EventRepo.AddExDate(r.Context(), *existingEvent.MasterEventUID, *existingEvent.OriginalStartTs); err != nil {
					logger.Warn("Failed to add exdate: %v", err)
				}
			}

			// Cancel/delete the instance
			if err := h.EventRepo.CancelInstance(r.Context(), eventUID); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to delete instance",
					err.Error(),
				)
				return
			}

			w.WriteHeader(http.StatusNoContent)

			// Queue webhook delivery
			if err := h.WebhookDispatcher.QueueDelivery(
				r.Context(),
				workers.EventDeleted,
				map[string]interface{}{
					"event_uid": eventUID.String(),
					"scope":     "single",
				},
				nil,
			); err != nil {
				logger.Error("Failed to queue webhook for event deletion: %v", err)
			}

		case ScopeAll:
			// Delete entire series - delete the master (cascades to all instances)
			if existingEvent.MasterEventUID == nil {
				web.RespondError(w, http.StatusBadRequest, "Instance has no master event")
				return
			}

			// Count instances before deletion for batch webhook
			instanceCount, err := h.EventRepo.CountInstancesByMaster(
				r.Context(),
				*existingEvent.MasterEventUID,
			)
			if err != nil {
				logger.Warn("Failed to count instances for batch webhook: %v", err)
				instanceCount = 0
			}

			if err := h.EventRepo.DeleteEvent(r.Context(), *existingEvent.MasterEventUID); err != nil {
				web.RespondError(
					w,
					http.StatusInternalServerError,
					"Failed to delete series",
					err.Error(),
				)
				return
			}

			w.WriteHeader(http.StatusNoContent)

			// Queue webhook delivery
			if instanceCount > 1 {
				// Use batch delivery for series deletion
				batchData := make([]interface{}, instanceCount+1) // +1 for master
				for i := 0; i < instanceCount+1; i++ {
					batchData[i] = map[string]interface{}{
						"event_uid":        eventUID.String(),
						"master_event_uid": existingEvent.MasterEventUID.String(),
						"scope":            "all",
					}
				}
				if err := h.WebhookDispatcher.QueueBatchDelivery(
					r.Context(),
					workers.EventDeleted,
					batchData,
				); err != nil {
					logger.Error("Failed to queue batch webhook for event deletion: %v", err)
				}
			} else {
				// Single event delivery
				if err := h.WebhookDispatcher.QueueDelivery(
					r.Context(),
					workers.EventDeleted,
					map[string]interface{}{
						"event_uid":        eventUID.String(),
						"master_event_uid": existingEvent.MasterEventUID.String(),
						"scope":            "all",
					},
					nil,
				); err != nil {
					logger.Error("Failed to queue webhook for event deletion: %v", err)
				}
			}

		default:
			web.RespondError(
				w,
				http.StatusBadRequest,
				"Invalid scope. Use 'single' or 'all' for delete operations",
			)
		}
		return
	}

	// Non-recurring event or master event - delete directly
	if err := h.EventRepo.DeleteEvent(r.Context(), eventUID); err != nil {
		web.RespondError(w, http.StatusInternalServerError, "Failed to delete event", err.Error())
		return
	}

	web.ResponseSuccess(w)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.EventDeleted,
		map[string]interface{}{
			"event_uid": eventUID.String(),
		},
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for event deletion: %v", err)
	}
}

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	eventUID, err := uuid.Parse(r.PathValue("event_uid"))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid event UID", err.Error())
		return
	}

	evt, err := h.EventRepo.GetEvent(r.Context(), eventUID, &[]bool{false}[0])
	if err != nil {
		web.RespondError(w, http.StatusNotFound, "Event not found", err.Error())
		return
	}

	web.RespondJSON(w, http.StatusOK, evt)
}

// GetCalendarEventsRequest represents the request body for querying calendar events
type GetCalendarEventsRequest struct {
	CalendarUIDs []string `json:"calendar_uids"`
	StartTs      int64    `json:"start_ts"`
	EndTs        int64    `json:"end_ts"`
}

func (h *Handler) GetCalendarEvents(w http.ResponseWriter, r *http.Request) {
	var req GetCalendarEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if len(req.CalendarUIDs) == 0 {
		web.RespondError(
			w,
			http.StatusBadRequest,
			"calendar_uids is required and must not be empty",
		)
		return
	}

	if req.StartTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "start_ts is required and must be positive")
		return
	}

	if req.EndTs <= 0 {
		web.RespondError(w, http.StatusBadRequest, "end_ts is required and must be positive")
		return
	}

	if req.EndTs <= req.StartTs {
		web.RespondError(w, http.StatusBadRequest, "end_ts must be greater than start_ts")
		return
	}

	// Parse calendar UIDs
	calendarUIDs := make([]uuid.UUID, 0, len(req.CalendarUIDs))
	for _, uidStr := range req.CalendarUIDs {
		calendarUID, err := uuid.Parse(uidStr)
		if err != nil {
			web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID: "+uidStr, err.Error())
			return
		}
		calendarUIDs = append(calendarUIDs, calendarUID)
	}

	// Verify all calendars exist
	for _, calendarUID := range calendarUIDs {
		_, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
		if err != nil {
			logger.Warn("Calendar not found: %v", err)
			web.RespondError(w, http.StatusNotFound, "Calendar not found: "+calendarUID.String())
			return
		}
	}

	events, err := h.EventRepo.GetCalendarEvents(r.Context(), calendarUIDs, req.StartTs, req.EndTs)
	if err != nil {
		web.RespondError(w, http.StatusInternalServerError, "Failed to get events", err.Error())
		return
	}

	web.ResponsePagedResults(w, events, len(events), nil, nil)
}
