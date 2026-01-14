package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/ics"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// ICSImportSummary contains summary statistics for an import
type ICSImportSummary struct {
	TotalEvents    int `json:"total_events"`
	ImportedEvents int `json:"imported_events"`
	FailedEvents   int `json:"failed_events"`
}

// ICSImportEventResult contains the result for a single event import
type ICSImportEventResult struct {
	ICSUID   string     `json:"ics_uid"`
	EventUID *uuid.UUID `json:"event_uid,omitempty"`
	Status   string     `json:"status"` // success, failed
	Error    *string    `json:"error,omitempty"`
}

// ICSImportResponse is the response for an ICS import
type ICSImportResponse struct {
	Calendar *calendar.Calendar     `json:"calendar"`
	Summary  ICSImportSummary       `json:"summary"`
	Events   []ICSImportEventResult `json:"events"`
}

// ImportICS handles ICS file upload and import
func (h *Handler) ImportICS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		logger.Warn("Failed to parse multipart form: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Failed to parse form data", err.Error())
		return
	}

	// Get required fields
	accountID := r.FormValue("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	// Get file
	file, _, err := r.FormFile("file")
	if err != nil {
		logger.Warn("Failed to get file from form: %v", err)
		web.RespondError(w, http.StatusBadRequest, "file is required", err.Error())
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Warn("Failed to close file: %v", err)
		}
	}()

	// Get optional fields
	calendarUIDStr := r.FormValue("calendar_uid")
	calendarMetadataStr := r.FormValue("calendar_metadata")
	importRemindersStr := r.FormValue("import_reminders")
	importAttendeesStr := r.FormValue("import_attendees")

	// Parse boolean options (default to true)
	importReminders := importRemindersStr != "false"

	importAttendees := importAttendeesStr != "false"

	// Determine target calendar
	var targetCalendar *calendar.Calendar

	if calendarUIDStr != "" {
		// Import to existing calendar
		calendarUID, err := uuid.Parse(calendarUIDStr)
		if err != nil {
			web.RespondError(w, http.StatusBadRequest, "Invalid calendar_uid", err.Error())
			return
		}

		targetCalendar, err = h.CalendarRepo.GetCalendar(ctx, calendarUID)
		if err != nil {
			logger.Warn("Calendar not found: %v", err)
			web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
			return
		}
	} else {
		// Create new calendar
		var metadata json.RawMessage
		if calendarMetadataStr != "" {
			// Validate JSON
			if !json.Valid([]byte(calendarMetadataStr)) {
				web.RespondError(
					w,
					http.StatusBadRequest,
					"calendar_metadata must be valid JSON",
				)
				return
			}
			metadata = json.RawMessage(calendarMetadataStr)
		} else {
			metadata = json.RawMessage(`{}`)
		}

		now := time.Now().Unix()
		newCalendar := &calendar.Calendar{
			CalendarUID: uuid.New(),
			AccountID:   accountID,
			Settings:    json.RawMessage(`{}`),
			Metadata:    metadata,
			CreatedTs:   now,
			UpdatedTs:   now,
		}

		if err := h.CalendarRepo.CreateCalendar(ctx, newCalendar); err != nil {
			logger.Error("Failed to create calendar: %v", err)
			web.RespondError(
				w,
				http.StatusInternalServerError,
				"Failed to create calendar",
				err.Error(),
			)
			return
		}

		targetCalendar = newCalendar

		// Queue webhook delivery for calendar creation
		if err := h.WebhookDispatcher.QueueDelivery(
			ctx,
			workers.CalendarCreated,
			newCalendar,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for calendar creation: %v", err)
		}
	}

	// Parse ICS file
	parser := ics.NewParser()
	icsCalendar, err := parser.ParseICS(file)
	if err != nil {
		logger.Warn("Failed to parse ICS file: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Failed to parse ICS file", err.Error())
		return
	}

	// Convert and import events
	converter := ics.NewConverter()
	response := ICSImportResponse{
		Calendar: targetCalendar,
		Summary: ICSImportSummary{
			TotalEvents: len(icsCalendar.Events),
		},
		Events: make([]ICSImportEventResult, 0, len(icsCalendar.Events)),
	}

	for _, icsEvent := range icsCalendar.Events {
		result := ICSImportEventResult{
			ICSUID: icsEvent.UID,
			Status: "success",
		}

		// Convert to internal format
		converted, err := converter.ConvertToEvent(icsEvent, targetCalendar.CalendarUID, accountID)
		if err != nil {
			result.Status = "failed"
			errMsg := err.Error()
			result.Error = &errMsg
			response.Summary.FailedEvents++
			response.Events = append(response.Events, result)
			continue
		}

		// Create event (with instances if recurring)
		var createdEvent *event.Event
		if converted.Event.Recurrence != nil {
			master, _, err := h.EventRepo.CreateEventWithInstances(
				ctx,
				converted.Event,
				h.GenerationWindow(),
			)
			if err != nil {
				result.Status = "failed"
				errMsg := err.Error()
				result.Error = &errMsg
				response.Summary.FailedEvents++
				response.Events = append(response.Events, result)
				continue
			}
			createdEvent = master
		} else {
			created, err := h.EventRepo.CreateEvent(ctx, converted.Event)
			if err != nil {
				result.Status = "failed"
				errMsg := err.Error()
				result.Error = &errMsg
				response.Summary.FailedEvents++
				response.Events = append(response.Events, result)
				continue
			}
			createdEvent = &created
		}

		result.EventUID = &createdEvent.EventUID

		// Import reminders
		if importReminders && len(converted.Reminders) > 0 {
			for _, r := range converted.Reminders {
				reminder := &event.Reminder{
					ReminderUID:   uuid.New(),
					EventUID:      createdEvent.EventUID,
					AccountID:     accountID,
					OffsetSeconds: r.OffsetSeconds,
					Metadata:      r.Metadata,
					CreatedTs:     time.Now().Unix(),
				}
				if _, err := h.ReminderRepo.CreateSingleReminder(ctx, reminder); err != nil {
					logger.Warn(
						"Failed to create reminder for event %s: %v",
						createdEvent.EventUID,
						err,
					)
				}
			}
		}

		// Import attendees
		if importAttendees && len(converted.Attendees) > 0 {
			for _, a := range converted.Attendees {
				if _, err := h.AttendeeRepo.CreateSingleAttendee(
					ctx,
					createdEvent.EventUID,
					a.AccountID,
					a.Role,
					a.Metadata,
				); err != nil {
					logger.Warn(
						"Failed to create attendee for event %s: %v",
						createdEvent.EventUID,
						err,
					)
				}
			}
		}

		response.Summary.ImportedEvents++
		response.Events = append(response.Events, result)

		// Queue webhook delivery for event creation
		if err := h.WebhookDispatcher.QueueDelivery(
			ctx,
			workers.EventCreated,
			createdEvent,
			nil,
		); err != nil {
			logger.Error("Failed to queue webhook for event creation: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, response)
}
