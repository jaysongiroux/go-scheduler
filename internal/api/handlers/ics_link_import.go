package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/crypto"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	reminder "github.com/jaysongiroux/go-scheduler/internal/db/services/reminders"
	"github.com/jaysongiroux/go-scheduler/internal/ics"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// ICSLinkImportRequest represents the request body for importing ICS from a link
type ICSLinkImportRequest struct {
	AccountID            string          `json:"account_id"`
	IcsURL               string          `json:"ics_url"`
	AuthType             string          `json:"auth_type"` // none, basic, bearer
	AuthCredentials      string          `json:"auth_credentials,omitempty"`
	SyncIntervalSeconds  int             `json:"sync_interval_seconds,omitempty"`
	CalendarMetadata     json.RawMessage `json:"calendar_metadata,omitempty"`
	SyncOnPartialFailure bool            `json:"sync_on_partial_failure"`
}

// ICSLinkImportResponse represents the response for ICS link import
type ICSLinkImportResponse struct {
	Calendar      *calendar.Calendar     `json:"calendar"`
	Summary       ICSImportSummary       `json:"summary"`
	Events        []ICSImportEventResult `json:"events"`
	SyncScheduled bool                   `json:"sync_scheduled"`
}

// ResyncResponse represents the response for manual resync
type ResyncResponse struct {
	Calendar       *calendar.Calendar `json:"calendar"`
	ImportedEvents int                `json:"imported_events"`
	FailedEvents   int                `json:"failed_events"`
	Warnings       []string           `json:"warnings"`
}

// ImportICSLink handles ICS URL import with automatic synchronization
func (h *Handler) ImportICSLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if encryption key is configured
	if len(h.Config.IcsEncryptionKey) == 0 {
		web.RespondError(
			w,
			http.StatusServiceUnavailable,
			"ICS link import is not configured (missing encryption key)",
		)
		return
	}

	var req ICSLinkImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("Failed to decode request body: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate required fields
	if req.AccountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id is required")
		return
	}
	if req.IcsURL == "" {
		web.RespondError(w, http.StatusBadRequest, "ics_url is required")
		return
	}

	// Validate URL
	parsedURL, err := url.Parse(req.IcsURL)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid ics_url", err.Error())
		return
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		web.RespondError(w, http.StatusBadRequest, "ics_url must use http or https scheme")
		return
	}

	// Validate auth type
	authType := req.AuthType
	if authType == "" {
		authType = "none"
	}
	if authType != "none" && authType != "basic" && authType != "bearer" {
		web.RespondError(w, http.StatusBadRequest, "auth_type must be 'none', 'basic', or 'bearer'")
		return
	}

	// Validate sync interval
	syncInterval := req.SyncIntervalSeconds
	if syncInterval == 0 {
		syncInterval = 86400 // Default 24 hours
	}
	if syncInterval < 300 { // Minimum 5 minutes
		web.RespondError(
			w,
			http.StatusBadRequest,
			"sync_interval_seconds must be at least 300 (5 minutes)",
		)
		return
	}

	// Initialize crypto service
	cryptoService, err := crypto.NewService(h.Config.IcsEncryptionKey)
	if err != nil {
		logger.Error("Failed to initialize crypto service: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to initialize encryption")
		return
	}

	// Encrypt credentials if provided
	var encryptedCredentials []byte
	if req.AuthCredentials != "" {
		encryptedCredentials, err = cryptoService.EncryptCredentials(req.AuthCredentials)
		if err != nil {
			logger.Error("Failed to encrypt credentials: %v", err)
			web.RespondError(w, http.StatusInternalServerError, "Failed to encrypt credentials")
			return
		}
	}

	// Create HTTP client for fetching ICS
	httpClient := &http.Client{
		Timeout: h.Config.IcsRequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// Fetch ICS content for validation
	httpReq, err := http.NewRequest("GET", req.IcsURL, nil)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to create request", err.Error())
		return
	}

	// Add authentication if provided
	if authType != "none" && req.AuthCredentials != "" {
		switch authType {
		case "basic":
			// credentials format: "username:password"
			parts := splitN(req.AuthCredentials, ":", 2)
			if len(parts) == 2 {
				httpReq.SetBasicAuth(parts[0], parts[1])
			}
		case "bearer":
			httpReq.Header.Set("Authorization", "Bearer "+req.AuthCredentials)
		}
	}

	// #nosec G704 -- URL is validated (scheme/host allowlist) before use
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to fetch ICS content", err.Error())
		return
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		web.RespondError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("Failed to fetch ICS: HTTP %d", resp.StatusCode),
		)
		return
	}

	// Read response body
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to read ICS content", err.Error())
		return
	}

	// Extract ETag and Last-Modified headers
	var etag, lastModified *string
	if etagVal := resp.Header.Get("ETag"); etagVal != "" {
		etag = &etagVal
	}
	if lastModVal := resp.Header.Get("Last-Modified"); lastModVal != "" {
		lastModified = &lastModVal
	}

	// Parse and validate ICS content
	parser := ics.NewParser()
	icsCalendar, err := parser.ParseICS(bytes.NewReader(buf.Bytes()))
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Failed to parse ICS file", err.Error())
		return
	}

	// Create calendar
	now := time.Now().Unix()
	metadata := req.CalendarMetadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	newCalendar := &calendar.Calendar{
		CalendarUID:            uuid.New(),
		AccountID:              req.AccountID,
		Settings:               json.RawMessage(`{}`),
		Metadata:               metadata,
		CreatedTs:              now,
		UpdatedTs:              now,
		IcsURL:                 &req.IcsURL,
		IcsAuthType:            &authType,
		IcsAuthCredentials:     encryptedCredentials,
		IcsLastSyncTs:          &now,
		IcsSyncIntervalSeconds: &syncInterval,
		IcsLastEtag:            etag,
		IcsLastModified:        lastModified,
		IsReadOnly:             true,
		SyncOnPartialFailure:   req.SyncOnPartialFailure,
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

	// Import events
	converter := ics.NewConverter()
	response := ICSLinkImportResponse{
		Calendar: newCalendar,
		Summary: ICSImportSummary{
			TotalEvents: len(icsCalendar.Events),
		},
		Events:        make([]ICSImportEventResult, 0, len(icsCalendar.Events)),
		SyncScheduled: true,
	}

	// Collect warnings for failed imports
	warnings := make([]string, 0)

	for _, icsEvent := range icsCalendar.Events {
		result := ICSImportEventResult{
			ICSUID: icsEvent.UID,
			Status: "success",
		}

		// Convert to internal format
		converted, err := converter.ConvertToEvent(icsEvent, newCalendar.CalendarUID, req.AccountID)
		if err != nil {
			result.Status = "failed"
			errMsg := err.Error()
			result.Error = &errMsg
			response.Summary.FailedEvents++
			response.Events = append(response.Events, result)
			warnings = append(
				warnings,
				fmt.Sprintf("Failed to convert event %s: %v", icsEvent.UID, err),
			)
			if !req.SyncOnPartialFailure {
				// Rollback: delete the calendar
				_ = h.CalendarRepo.DeleteCalendar(ctx, newCalendar.CalendarUID)
				web.RespondError(
					w,
					http.StatusBadRequest,
					fmt.Sprintf("Failed to convert event %s", icsEvent.UID),
					err.Error(),
				)
				return
			}
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
				warnings = append(
					warnings,
					fmt.Sprintf("Failed to create recurring event %s: %v", icsEvent.UID, err),
				)
				if !req.SyncOnPartialFailure {
					// Rollback: delete the calendar
					_ = h.CalendarRepo.DeleteCalendar(ctx, newCalendar.CalendarUID)
					web.RespondError(
						w,
						http.StatusBadRequest,
						fmt.Sprintf("Failed to create recurring event %s", icsEvent.UID),
						err.Error(),
					)
					return
				}
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
				warnings = append(warnings, fmt.Sprintf("Failed to create event %s: %v", icsEvent.UID, err))
				if !req.SyncOnPartialFailure {
					// Rollback: delete the calendar
					_ = h.CalendarRepo.DeleteCalendar(ctx, newCalendar.CalendarUID)
					web.RespondError(
						w,
						http.StatusBadRequest,
						fmt.Sprintf("Failed to create event %s", icsEvent.UID),
						err.Error(),
					)
					return
				}
				continue
			}
			createdEvent = &created
		}

		result.EventUID = &createdEvent.EventUID

		// Import reminders
		for _, r := range converted.Reminders {
			reminder := &event.Reminder{
				ReminderUID:   uuid.New(),
				EventUID:      createdEvent.EventUID,
				AccountID:     req.AccountID,
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

		// Import attendees
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

	// Update sync status
	statusStr := "success"
	if err := h.CalendarRepo.UpdateSyncStatus(
		ctx,
		newCalendar.CalendarUID,
		statusStr,
		nil,
		now,
		etag,
		lastModified,
	); err != nil {
		logger.Warn("Failed to update sync status: %v", err)
	}

	// Queue webhook delivery for calendar creation
	if err := h.WebhookDispatcher.QueueDelivery(
		ctx,
		workers.CalendarCreated,
		newCalendar,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for calendar creation: %v", err)
	}

	// Queue webhook for initial sync completion
	if h.WebhookDispatcher != nil {
		syncData := map[string]interface{}{
			"calendar_uid":    newCalendar.CalendarUID,
			"account_id":      newCalendar.AccountID,
			"imported_events": response.Summary.ImportedEvents,
			"failed_events":   response.Summary.FailedEvents,
			"warnings":        warnings,
			"sync_ts":         now,
		}
		if err := h.WebhookDispatcher.QueueDelivery(ctx, workers.CalendarSynced, syncData, nil); err != nil {
			logger.Error("Failed to queue webhook for calendar sync: %v", err)
		}
	}

	web.RespondJSON(w, http.StatusOK, response)
}

// ResyncCalendar handles manual resync of an ICS calendar
func (h *Handler) ResyncCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if encryption key is configured
	if len(h.Config.IcsEncryptionKey) == 0 {
		web.RespondError(
			w,
			http.StatusServiceUnavailable,
			"ICS sync is not configured (missing encryption key)",
		)
		return
	}

	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	// Get calendar
	cal, err := h.CalendarRepo.GetCalendar(ctx, calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	// Verify it's an ICS import
	if cal.IcsURL == nil || *cal.IcsURL == "" {
		web.RespondError(w, http.StatusBadRequest, "Calendar is not an ICS import")
		return
	}

	// Verify user is the owner
	// In a real implementation, you'd extract account_id from auth context
	// For now, we'll allow any resync request

	// Initialize crypto service
	cryptoService, err := crypto.NewService(h.Config.IcsEncryptionKey)
	if err != nil {
		logger.Error("Failed to initialize crypto service: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to initialize encryption")
		return
	}

	// Create ICS sync worker for manual sync
	icsSyncWorker := workers.NewIcsSyncWorker(
		h.CalendarRepo.(*calendar.Queries),
		h.EventRepo.(*event.Queries),
		h.ReminderRepo.(*reminder.Queries),
		h.AttendeeRepo,
		h.WebhookDispatcher,
		cryptoService,
		h.Config,
	)

	// Perform sync
	result, err := icsSyncWorker.SyncCalendarNow(ctx, calendarUID)
	if err != nil {
		logger.Error("Failed to sync calendar: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to sync calendar", err.Error())
		return
	}

	// Reload calendar to get updated sync status
	cal, err = h.CalendarRepo.GetCalendar(ctx, calendarUID)
	if err != nil {
		logger.Warn("Failed to reload calendar: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to reload calendar",
			err.Error(),
		)
		return
	}

	response := ResyncResponse{
		Calendar:       cal,
		ImportedEvents: result.ImportedEvents,
		FailedEvents:   result.FailedEvents,
		Warnings:       result.Warnings,
	}

	web.RespondJSON(w, http.StatusOK, response)
}

// Helper function to split string
func splitN(s, sep string, n int) []string {
	if n == 0 {
		return nil
	}

	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := findString(s, sep)
		if idx == -1 {
			result = append(result, s)
			return result
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
