package workers

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// ReminderRepository defines the interface for reminder operations needed by the worker
type ReminderRepository interface {
	GetDueRemindersWithEvents(ctx context.Context, beforeTs int64, limit int) ([]*event.ReminderWithEvent, error)
	MarkReminderDelivered(ctx context.Context, reminderUID uuid.UUID) error
}

type ReminderTriggerWorker struct {
	reminderRepo      ReminderRepository
	webhookDispatcher *WebhookDispatcher
	pollInterval      time.Duration
	batchSize         int
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

func NewReminderTriggerWorker(
	reminderRepo ReminderRepository,
	webhookDispatcher *WebhookDispatcher,
	pollInterval time.Duration,
	batchSize int,
) *ReminderTriggerWorker {
	return &ReminderTriggerWorker{
		reminderRepo:      reminderRepo,
		webhookDispatcher: webhookDispatcher,
		pollInterval:      pollInterval,
		batchSize:         batchSize,
		stopCh:            make(chan struct{}),
	}
}

func (w *ReminderTriggerWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	defer w.wg.Done()

	logger.Info("Starting reminder trigger worker (poll interval: %v)", w.pollInterval)

	// Process immediately on start
	w.processReminders(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Reminder trigger worker stopping (context cancelled)")
			return
		case <-w.stopCh:
			logger.Info("Reminder trigger worker stopping (stop signal)")
			return
		case <-ticker.C:
			w.processReminders(ctx)
		}
	}
}

func (w *ReminderTriggerWorker) Stop() {
	logger.Info("Stopping reminder trigger worker...")
	close(w.stopCh)
	w.wg.Wait()
}

func (w *ReminderTriggerWorker) processReminders(ctx context.Context) {
	now := time.Now().UTC().Unix()

	remindersWithEvents, err := w.reminderRepo.GetDueRemindersWithEvents(ctx, now, w.batchSize)
	if err != nil {
		logger.Error("Failed to fetch due reminders: %v", err)
		return
	}

	if len(remindersWithEvents) == 0 {
		return
	}

	logger.Info("Processing %d due reminders", len(remindersWithEvents))

	// Use batch delivery for webhooks
	batchData := make([]interface{}, 0, len(remindersWithEvents))
	for _, rwe := range remindersWithEvents {
		batchData = append(batchData, map[string]interface{}{
			"event": map[string]interface{}{
				"event_uid":    rwe.EventUID,
				"calendar_uid": rwe.CalendarUID,
				"account_id":   rwe.AccountID,
				"start_ts":     rwe.StartTs,
				"metadata":     rwe.EventMetadata,
			},
			"reminder": map[string]interface{}{
				"reminder_uid":   rwe.ReminderUID,
				"offset_seconds": rwe.OffsetSeconds,
				"metadata":       rwe.ReminderMetadata,
				"remind_at_ts":   rwe.RemindAtTs,
			},
		})
	}

	// Queue batch webhook delivery
	if err := w.webhookDispatcher.QueueBatchDelivery(
		ctx,
		ReminderTriggered,
		batchData,
	); err != nil {
		logger.Error("Failed to queue batch webhook for reminders: %v", err)
		return
	}

	// Mark all as delivered
	successCount := 0
	failCount := 0
	for _, rwe := range remindersWithEvents {
		if err := w.reminderRepo.MarkReminderDelivered(ctx, rwe.ReminderUID); err != nil {
			logger.Error("Failed to mark reminder %s as delivered: %v", rwe.ReminderUID, err)
			failCount++
			continue
		}
		successCount++
	}

	logger.Info("Processed %d reminders: %d succeeded, %d failed",
		len(remindersWithEvents), successCount, failCount)
}
