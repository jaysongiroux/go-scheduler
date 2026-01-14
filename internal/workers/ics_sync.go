package workers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/crypto"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/attendee"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	reminders "github.com/jaysongiroux/go-scheduler/internal/db/services/reminders"
	"github.com/jaysongiroux/go-scheduler/internal/ics"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// IcsSyncWorker handles periodic synchronization of ICS calendar imports
type IcsSyncWorker struct {
	calendarRepo      *calendar.Queries
	eventRepo         *event.Queries
	reminderRepo      *reminders.Queries
	attendeeRepo      *attendee.Queries
	webhookDispatcher *WebhookDispatcher
	crypto            *crypto.Service
	config            *config.Config
	httpClient        *http.Client
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

// NewIcsSyncWorker creates a new ICS sync worker
func NewIcsSyncWorker(
	calendarRepo *calendar.Queries,
	eventRepo *event.Queries,
	reminderRepo *reminders.Queries,
	attendeeRepo *attendee.Queries,
	webhookDispatcher *WebhookDispatcher,
	cryptoService *crypto.Service,
	cfg *config.Config,
) *IcsSyncWorker {
	return &IcsSyncWorker{
		calendarRepo:      calendarRepo,
		eventRepo:         eventRepo,
		reminderRepo:      reminderRepo,
		attendeeRepo:      attendeeRepo,
		webhookDispatcher: webhookDispatcher,
		crypto:            cryptoService,
		config:            cfg,
		httpClient: &http.Client{
			Timeout: cfg.IcsRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins the ICS sync worker
func (w *IcsSyncWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	defer w.wg.Done()

	logger.Info("Starting ICS sync worker (interval: %v)", w.config.IcsSyncInterval)

	// Run immediately on start
	w.processCalendarsForSync(ctx)

	ticker := time.NewTicker(w.config.IcsSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("ICS sync worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			logger.Info("ICS sync worker stopping (stop signal)")
			return
		case <-ticker.C:
			w.processCalendarsForSync(ctx)
		}
	}
}

// Stop signals the worker to stop
func (w *IcsSyncWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// processCalendarsForSync finds and syncs calendars that need updating
func (w *IcsSyncWorker) processCalendarsForSync(ctx context.Context) {
	logger.Debug("ICS sync worker: checking for calendars needing sync")

	totalProcessed := 0
	totalSuccess := 0
	totalFailed := 0

	for {
		// Get batch of calendars needing sync (with row locking)
		calendars, err := w.calendarRepo.GetCalendarsNeedingSync(ctx, w.config.IcsSyncBatchSize)
		if err != nil {
			logger.Error("ICS sync worker: failed to get calendars needing sync: %v", err)
			return
		}

		if len(calendars) == 0 {
			break
		}

		for _, cal := range calendars {
			if err := w.syncCalendar(ctx, cal); err != nil {
				logger.Error(
					"ICS sync worker: failed to sync calendar %s: %v",
					cal.CalendarUID,
					err,
				)
				totalFailed++
			} else {
				totalSuccess++
			}
			totalProcessed++
		}

		// If we got less than batchSize, we're done
		if len(calendars) < w.config.IcsSyncBatchSize {
			break
		}
	}

	if totalProcessed > 0 {
		logger.Info(
			"ICS sync worker: processed %d calendars (%d success, %d failed)",
			totalProcessed,
			totalSuccess,
			totalFailed,
		)
	} else {
		logger.Debug("ICS sync worker: no calendars needed sync")
	}
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	ImportedEvents int
	FailedEvents   int
	Warnings       []string
}

// syncCalendar synchronizes a single calendar from its ICS source
func (w *IcsSyncWorker) syncCalendar(ctx context.Context, cal *calendar.Calendar) error {
	logger.Debug("Syncing calendar %s from %s", cal.CalendarUID, *cal.IcsURL)

	now := time.Now().Unix()

	// Fetch ICS content
	icsContent, etag, lastModified, err := w.fetchICSContent(cal)
	if err != nil {
		// Update status to stale but keep existing events
		errMsg := err.Error()
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"stale",
			&errMsg,
			now,
			nil,
			nil,
		)
		return fmt.Errorf("failed to fetch ICS content: %w", err)
	}

	// Parse ICS content
	parser := ics.NewParser()
	icsCalendar, err := parser.ParseICS(bytes.NewReader(icsContent))
	if err != nil {
		errMsg := fmt.Sprintf("failed to parse ICS: %v", err)
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"stale",
			&errMsg,
			now,
			etag,
			lastModified,
		)
		return fmt.Errorf("failed to parse ICS: %w", err)
	}

	// Perform full replace sync
	result, err := w.fullReplaceSync(ctx, cal, icsCalendar.Events)
	if err != nil {
		errMsg := fmt.Sprintf("sync failed: %v", err)
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"failed",
			&errMsg,
			now,
			etag,
			lastModified,
		)
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Update sync status
	var errorMsg *string
	if len(result.Warnings) > 0 {
		warningsStr := strings.Join(result.Warnings, "; ")
		errorMsg = &warningsStr
	}

	if err := w.calendarRepo.UpdateSyncStatus(
		ctx,
		cal.CalendarUID,
		"success",
		errorMsg,
		now,
		etag,
		lastModified,
	); err != nil {
		logger.Warn("Failed to update sync status for calendar %s: %v", cal.CalendarUID, err)
	}

	logger.Info(
		"Successfully synced calendar %s: %d events imported, %d failed",
		cal.CalendarUID,
		result.ImportedEvents,
		result.FailedEvents,
	)

	// Queue webhook for calendar sync
	if w.webhookDispatcher != nil {
		syncData := map[string]interface{}{
			"calendar_uid":    cal.CalendarUID,
			"account_id":      cal.AccountID,
			"imported_events": result.ImportedEvents,
			"failed_events":   result.FailedEvents,
			"warnings":        result.Warnings,
			"sync_ts":         now,
		}
		if err := w.webhookDispatcher.QueueDelivery(ctx, CalendarSynced, syncData, nil); err != nil {
			logger.Error("Failed to queue webhook for calendar sync: %v", err)
		}
	}

	return nil
}

// fetchICSContent fetches ICS content from the URL with authentication
func (w *IcsSyncWorker) fetchICSContent(cal *calendar.Calendar) ([]byte, *string, *string, error) {
	if cal.IcsURL == nil || *cal.IcsURL == "" {
		return nil, nil, nil, fmt.Errorf("calendar has no ICS URL")
	}

	req, err := http.NewRequest("GET", *cal.IcsURL, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add If-Modified-Since header if available
	if cal.IcsLastModified != nil && *cal.IcsLastModified != "" {
		req.Header.Set("If-Modified-Since", *cal.IcsLastModified)
	}

	// Add authentication if configured
	if cal.IcsAuthType != nil && *cal.IcsAuthType != "none" && len(cal.IcsAuthCredentials) > 0 {
		credentials, err := w.crypto.DecryptCredentials(cal.IcsAuthCredentials)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		switch *cal.IcsAuthType {
		case "basic":
			// credentials format: "username:password"
			parts := strings.SplitN(credentials, ":", 2)
			if len(parts) == 2 {
				req.SetBasicAuth(parts[0], parts[1])
			}
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+credentials)
		}
	}

	// #nosec G704 -- URL is from validated ICS link config (scheme/host allowlist)
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch ICS: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body: %v", err)
		}
	}()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		return nil, nil, nil, fmt.Errorf("not modified (304)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Extract ETag and Last-Modified headers
	var etag, lastModified *string
	if etagVal := resp.Header.Get("ETag"); etagVal != "" {
		etag = &etagVal
	}
	if lastModVal := resp.Header.Get("Last-Modified"); lastModVal != "" {
		lastModified = &lastModVal
	}

	return buf.Bytes(), etag, lastModified, nil
}

// fullReplaceSync deletes all events and re-imports from ICS
func (w *IcsSyncWorker) fullReplaceSync(
	ctx context.Context,
	cal *calendar.Calendar,
	icsEvents []ics.ICSEvent,
) (*SyncResult, error) {
	result := &SyncResult{
		Warnings: make([]string, 0),
	}

	// Begin transaction
	tx, err := w.eventRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil {
			logger.Warn("Failed to rollback transaction: %v", err)
		}
	}()

	// Delete all existing events for this calendar
	deleteQuery := `DELETE FROM calendar_events WHERE calendar_uid = $1`
	if _, err := tx.ExecContext(ctx, deleteQuery, cal.CalendarUID); err != nil {
		return nil, fmt.Errorf("failed to delete existing events: %w", err)
	}

	// Convert and import events
	converter := ics.NewConverter()
	importedEvents := make([]*event.Event, 0)

	for _, icsEvent := range icsEvents {
		// Convert to internal format
		converted, err := converter.ConvertToEvent(icsEvent, cal.CalendarUID, cal.AccountID)
		if err != nil {
			result.FailedEvents++
			if cal.SyncOnPartialFailure {
				result.Warnings = append(
					result.Warnings,
					fmt.Sprintf("Failed to convert event %s: %v", icsEvent.UID, err),
				)
				continue
			} else {
				return nil, fmt.Errorf("failed to convert event %s: %w", icsEvent.UID, err)
			}
		}

		// Create event
		var createdEvent *event.Event
		if converted.Event.Recurrence != nil {
			// Create recurring event with instances
			window := event.NewGenerationWindow(
				w.config.GenerationWindow,
				w.config.GenerationBuffer,
			)
			master, _, err := w.eventRepo.CreateEventWithInstancesTx(
				ctx,
				tx,
				converted.Event,
				window,
			)
			if err != nil {
				result.FailedEvents++
				if cal.SyncOnPartialFailure {
					result.Warnings = append(
						result.Warnings,
						fmt.Sprintf("Failed to create recurring event %s: %v", icsEvent.UID, err),
					)
					continue
				} else {
					return nil, fmt.Errorf("failed to create recurring event %s: %w", icsEvent.UID, err)
				}
			}
			createdEvent = master
		} else {
			// Create single event
			created, err := w.eventRepo.CreateEventTx(ctx, tx, converted.Event)
			if err != nil {
				result.FailedEvents++
				if cal.SyncOnPartialFailure {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create event %s: %v", icsEvent.UID, err))
					continue
				} else {
					return nil, fmt.Errorf("failed to create event %s: %w", icsEvent.UID, err)
				}
			}
			createdEvent = &created
		}

		importedEvents = append(importedEvents, createdEvent)
		result.ImportedEvents++

		// Import reminders
		for _, r := range converted.Reminders {
			reminder := &event.Reminder{
				ReminderUID:   uuid.New(),
				EventUID:      createdEvent.EventUID,
				AccountID:     cal.AccountID,
				OffsetSeconds: r.OffsetSeconds,
				Metadata:      r.Metadata,
				CreatedTs:     time.Now().Unix(),
			}
			if err := w.reminderRepo.CreateSingleReminderTx(ctx, tx, reminder); err != nil {
				logger.Warn(
					"Failed to create reminder for event %s: %v",
					createdEvent.EventUID,
					err,
				)
			}
		}

		// Import attendees
		for _, a := range converted.Attendees {
			if err := w.attendeeRepo.CreateSingleAttendeeTx(
				ctx,
				tx,
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

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Queue batch webhook for created events
	if w.webhookDispatcher != nil && len(importedEvents) > 0 {
		batchData := make([]interface{}, len(importedEvents))
		for i, evt := range importedEvents {
			batchData[i] = map[string]interface{}{
				"event_uid":    evt.EventUID,
				"calendar_uid": evt.CalendarUID,
				"account_id":   evt.AccountID,
				"start_ts":     evt.StartTs,
				"end_ts":       evt.EndTs,
				"metadata":     evt.Metadata,
			}
		}
		if err := w.webhookDispatcher.QueueBatchDelivery(ctx, EventCreated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for imported events: %v", err)
		}
	}

	return result, nil
}

// SyncCalendarNow performs an immediate sync of a calendar (for manual resync endpoint)
func (w *IcsSyncWorker) SyncCalendarNow(
	ctx context.Context,
	calendarUID uuid.UUID,
) (*SyncResult, error) {
	cal, err := w.calendarRepo.GetCalendar(ctx, calendarUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get calendar: %w", err)
	}

	if cal.IcsURL == nil || *cal.IcsURL == "" {
		return nil, fmt.Errorf("calendar is not an ICS import")
	}

	// Fetch ICS content
	icsContent, etag, lastModified, err := w.fetchICSContent(cal)
	if err != nil {
		now := time.Now().Unix()
		errMsg := err.Error()
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"stale",
			&errMsg,
			now,
			nil,
			nil,
		)
		return nil, fmt.Errorf("failed to fetch ICS content: %w", err)
	}

	// Parse ICS content
	parser := ics.NewParser()
	icsCalendar, err := parser.ParseICS(bytes.NewReader(icsContent))
	if err != nil {
		now := time.Now().Unix()
		errMsg := fmt.Sprintf("failed to parse ICS: %v", err)
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"stale",
			&errMsg,
			now,
			etag,
			lastModified,
		)
		return nil, fmt.Errorf("failed to parse ICS: %w", err)
	}

	// Perform full replace sync
	result, err := w.fullReplaceSync(ctx, cal, icsCalendar.Events)
	if err != nil {
		now := time.Now().Unix()
		errMsg := fmt.Sprintf("sync failed: %v", err)
		_ = w.calendarRepo.UpdateSyncStatus(
			ctx,
			cal.CalendarUID,
			"failed",
			&errMsg,
			now,
			etag,
			lastModified,
		)
		return nil, fmt.Errorf("failed to sync: %w", err)
	}

	// Update sync status
	now := time.Now().Unix()
	var errorMsg *string
	if len(result.Warnings) > 0 {
		warningsStr := strings.Join(result.Warnings, "; ")
		errorMsg = &warningsStr
	}

	if err := w.calendarRepo.UpdateSyncStatus(
		ctx,
		cal.CalendarUID,
		"success",
		errorMsg,
		now,
		etag,
		lastModified,
	); err != nil {
		logger.Warn("Failed to update sync status for calendar %s: %v", cal.CalendarUID, err)
	}

	// Queue webhook for manual resync
	if w.webhookDispatcher != nil {
		syncData := map[string]interface{}{
			"calendar_uid":    cal.CalendarUID,
			"account_id":      cal.AccountID,
			"imported_events": result.ImportedEvents,
			"failed_events":   result.FailedEvents,
			"warnings":        result.Warnings,
			"sync_ts":         now,
			"manual_trigger":  true,
		}
		if err := w.webhookDispatcher.QueueDelivery(ctx, CalendarResynced, syncData, nil); err != nil {
			logger.Error("Failed to queue webhook for calendar resync: %v", err)
		}
	}

	return result, nil
}
