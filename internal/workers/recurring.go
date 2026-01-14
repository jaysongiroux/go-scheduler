package workers

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/attendee"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	reminders "github.com/jaysongiroux/go-scheduler/internal/db/services/reminders"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	rr "github.com/teambition/rrule-go"
)

// Helper wrappers for rrule-go
var parseRRule = rr.StrToROption
var newRRule = rr.NewRRule
var newUUID = uuid.New

// RecurringWorker handles background generation of recurring event instances
type RecurringWorker struct {
	eventRepo         *event.Queries
	reminderRepo      *reminders.Queries
	attendeeRepo      *attendee.Queries
	webhookDispatcher *WebhookDispatcher
	config            *config.Config
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

// NewRecurringWorker creates a new recurring event generation worker
func NewRecurringWorker(
	eventRepo *event.Queries,
	reminderRepo *reminders.Queries,
	attendeeRepo *attendee.Queries,
	webhookDispatcher *WebhookDispatcher,
	cfg *config.Config,
) *RecurringWorker {
	return &RecurringWorker{
		eventRepo:         eventRepo,
		reminderRepo:      reminderRepo,
		attendeeRepo:      attendeeRepo,
		webhookDispatcher: webhookDispatcher,
		config:            cfg,
		stopCh:            make(chan struct{}),
	}
}

// Start begins the recurring generation worker
func (w *RecurringWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	defer w.wg.Done()

	logger.Info(
		"Starting recurring generation worker (interval: %v)",
		w.config.RecurringGenerationInterval,
	)

	// Run immediately on start
	w.processActiveEvents(ctx)

	ticker := time.NewTicker(w.config.RecurringGenerationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Recurring generation worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			logger.Info("Recurring generation worker stopping (stop signal)")
			return
		case <-ticker.C:
			w.processActiveEvents(ctx)
		}
	}
}

// Stop signals the worker to stop
func (w *RecurringWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// processActiveEvents finds and processes all recurring events that need instance generation
func (w *RecurringWorker) processActiveEvents(ctx context.Context) {
	logger.Debug("Recurring worker: checking for events needing generation")

	// Process in batches to avoid memory issues
	batchSize := 1000
	totalProcessed := 0
	totalInstancesGenerated := 0

	for {
		// Get batch of active recurring events needing generation
		events, err := w.eventRepo.GetActiveRecurringEvents(ctx, batchSize)
		if err != nil {
			logger.Error("Recurring worker: failed to get active events: %v", err)
			return
		}

		if len(events) == 0 {
			break
		}

		for _, evt := range events {
			instances, err := w.generateInstancesForEvent(ctx, evt)
			if err != nil {
				logger.Error(
					"Recurring worker: failed to generate instances for event %s: %v",
					evt.EventUID,
					err,
				)
				continue
			}

			totalProcessed++
			totalInstancesGenerated += instances
		}

		// If we got less than batchSize, we're done
		if len(events) < batchSize {
			break
		}
	}

	if totalProcessed > 0 {
		logger.Info(
			"Recurring worker: processed %d events, generated %d instances",
			totalProcessed,
			totalInstancesGenerated,
		)
	} else {
		logger.Debug("Recurring worker: no events needed generation")
	}
}

// generateInstancesForEvent generates instances for a single master event
func (w *RecurringWorker) generateInstancesForEvent(
	ctx context.Context,
	master *event.Event,
) (int, error) {
	// Get the latest existing instance timestamp
	latestTs, err := w.eventRepo.GetLatestInstanceTimestamp(ctx, master.EventUID)
	if err != nil {
		return 0, err
	}

	// Calculate generation window
	now := time.Now().UTC()
	windowEnd := now.Add(w.config.GenerationWindow + w.config.GenerationBuffer)

	// Determine start of generation range
	var genStart int64
	if latestTs > 0 {
		// Start from the day after the latest instance
		genStart = latestTs + 1
	} else {
		// No instances yet, start from event start time
		genStart = master.StartTs
	}

	// Don't generate if we're already beyond the window
	if genStart >= windowEnd.Unix() {
		return 0, nil
	}

	// Generate instances
	window := event.GenerationWindow{
		WindowDuration: w.config.GenerationWindow,
		BufferDuration: w.config.GenerationBuffer,
	}

	instances := w.expandMasterEvent(master, genStart, windowEnd.Unix())

	if len(instances) == 0 {
		// No more instances to generate - check if series should be marked inactive
		if master.RecurrenceStatus != nil &&
			*master.RecurrenceStatus == event.RecurrenceStatusActive {
			// Check if the series has ended
			status := w.calculateRecurrenceStatus(master, window)
			if status == event.RecurrenceStatusInactive {
				if err := w.eventRepo.UpdateRecurrenceStatus(ctx, master.EventUID, event.RecurrenceStatusInactive); err != nil {
					logger.Warn(
						"Recurring worker: failed to update recurrence status for %s: %v",
						master.EventUID,
						err,
					)
				}
			}
		}
		return 0, nil
	}

	// Bulk insert instances
	if err := w.eventRepo.BulkInsertInstances(ctx, instances); err != nil {
		return 0, err
	}

	// Copy attendees and reminders for each generated instance
	totalAttendeesCopied := 0
	totalRemindersCopied := 0
	for _, inst := range instances {
		// Copy attendees from master to instance
		if w.attendeeRepo != nil {
			attendeeCount, err := w.attendeeRepo.CopyAttendeesToNewEvent(
				ctx,
				master.EventUID,
				inst.EventUID,
			)
			if err != nil {
				logger.Warn(
					"Recurring worker: failed to copy attendees for instance %s: %v",
					inst.EventUID,
					err,
				)
			} else {
				totalAttendeesCopied += attendeeCount
			}
		}

		// Copy reminders from master to instance
		if w.reminderRepo != nil {
			reminderCount, err := w.reminderRepo.CopyRemindersToNewEvent(
				ctx,
				master.EventUID,
				inst.EventUID,
			)
			if err != nil {
				logger.Warn(
					"Recurring worker: failed to copy reminders for instance %s: %v",
					inst.EventUID,
					err,
				)
			} else {
				totalRemindersCopied += reminderCount
			}
		}
	}

	if totalAttendeesCopied > 0 {
		logger.Debug(
			"Recurring worker: copied %d attendees for %d instances",
			totalAttendeesCopied,
			len(instances),
		)
	}
	if totalRemindersCopied > 0 {
		logger.Debug(
			"Recurring worker: copied %d reminders for %d instances",
			totalRemindersCopied,
			len(instances),
		)
	}

	// Queue batch webhook delivery for created instances
	if w.webhookDispatcher != nil && len(instances) > 0 {
		batchData := make([]interface{}, len(instances))
		for i, inst := range instances {
			batchData[i] = map[string]interface{}{
				"event_uid":        inst.EventUID,
				"calendar_uid":     inst.CalendarUID,
				"account_id":       inst.AccountID,
				"start_ts":         inst.StartTs,
				"end_ts":           inst.EndTs,
				"master_event_uid": inst.MasterEventUID,
				"metadata":         inst.Metadata,
			}
		}
		if err := w.webhookDispatcher.QueueBatchDelivery(ctx, EventCreated, batchData); err != nil {
			logger.Error("Failed to queue batch webhook for generated instances: %v", err)
		}
	}

	return len(instances), nil
}

// expandMasterEvent generates instance events from a master event within a time range
func (w *RecurringWorker) expandMasterEvent(
	master *event.Event,
	startTs, endTs int64,
) []*event.Event {
	// Use the event repository's internal generation logic
	// This is a simplified version - in production, you'd want to use rrule-go directly
	window := event.GenerationWindow{
		WindowDuration: w.config.GenerationWindow,
		BufferDuration: w.config.GenerationBuffer,
	}
	_ = window // Placeholder for now

	// Create a temporary Queries instance to use generateInstances
	// In a real implementation, this would be refactored to expose the generation logic
	return generateInstancesInternal(master, startTs, endTs)
}

// calculateRecurrenceStatus determines if a recurring event is active or inactive
func (w *RecurringWorker) calculateRecurrenceStatus(
	evt *event.Event,
	window event.GenerationWindow,
) event.RecurrenceStatus {
	if evt.RecurrenceEndTs != nil {
		windowEnd := time.Now().Add(window.WindowDuration)
		if *evt.RecurrenceEndTs < windowEnd.Unix() {
			return event.RecurrenceStatusInactive
		}
	}
	return event.RecurrenceStatusActive
}

// generateInstancesInternal creates instance events from a master event within a time range
// This mirrors the logic in the event repository
// Uses timezone-aware expansion to correctly handle DST transitions.
func generateInstancesInternal(master *event.Event, startTs, endTs int64) []*event.Event {
	if master == nil || master.Recurrence == nil || master.Recurrence.Rule == "" {
		return nil
	}

	// Use rrule-go to parse and expand
	opt, err := parseRRule(master.Recurrence.Rule)
	if err != nil {
		return nil
	}

	// Load timezone for DST-aware expansion (default to UTC if not set)
	loc := time.UTC
	if master.Timezone != nil && *master.Timezone != "" {
		loc, err = time.LoadLocation(*master.Timezone)
		if err != nil {
			logger.Warn(
				"Failed to load timezone %s, falling back to UTC: %v",
				*master.Timezone,
				err,
			)
			loc = time.UTC
		}
	}

	// Apply DTSTART in local timezone for DST-aware recurrence expansion
	if master.LocalStart != nil && *master.LocalStart != "" {
		localTime, err := time.ParseInLocation("2006-01-02T15:04:05", *master.LocalStart, loc)
		if err == nil {
			opt.Dtstart = localTime
		} else {
			opt.Dtstart = time.Unix(master.StartTs, 0).In(loc)
		}
	} else {
		opt.Dtstart = time.Unix(master.StartTs, 0).In(loc)
	}

	r, err := newRRule(*opt)
	if err != nil {
		return nil
	}

	// Expand occurrences in local timezone
	occurrences := r.Between(
		time.Unix(startTs, 0).In(loc),
		time.Unix(endTs, 0).In(loc),
		true,
	)

	// Build EXDATE lookup
	exDateMap := make(map[int64]bool, len(master.ExDatesTs))
	for _, ex := range master.ExDatesTs {
		exDateMap[ex] = true
	}

	now := time.Now().UTC().Unix()
	instances := make([]*event.Event, 0, len(occurrences))

	for _, occ := range occurrences {
		// Convert occurrence to UTC for storage
		occTs := occ.UTC().Unix()

		// Skip excluded instances
		if exDateMap[occTs] {
			continue
		}

		// Compute local_start for this occurrence (format in event TZ for DST consistency)
		var localStart *string
		if master.Timezone != nil && *master.Timezone != "" {
			ls := occ.In(loc).Format("2006-01-02T15:04:05")
			localStart = &ls
		}

		instance := &event.Event{
			EventUID:            newUUID(),
			CalendarUID:         master.CalendarUID,
			AccountID:           master.AccountID,
			StartTs:             occTs,
			Duration:            master.Duration,
			EndTs:               occTs + master.Duration,
			CreatedTs:           now,
			UpdatedTs:           now,
			Timezone:            master.Timezone,
			LocalStart:          localStart,
			IsRecurringInstance: true,
			MasterEventUID:      &master.EventUID,
			OriginalStartTs:     &occTs,
			Metadata:            master.Metadata,
		}

		instances = append(instances, instance)
	}

	return instances
}
