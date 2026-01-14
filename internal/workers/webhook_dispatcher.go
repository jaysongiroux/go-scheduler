package workers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/webhook"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	// event
	EventCreated  WebhookEventType = "event.created"
	EventUpdated  WebhookEventType = "event.updated"
	EventDeleted  WebhookEventType = "event.deleted"
	EventReminder WebhookEventType = "event.reminder"

	// calendar
	CalendarCreated  WebhookEventType = "calendar.created"
	CalendarUpdated  WebhookEventType = "calendar.updated"
	CalendarDeleted  WebhookEventType = "calendar.deleted"
	CalendarSynced   WebhookEventType = "calendar.synced"
	CalendarResynced WebhookEventType = "calendar.resynced"

	// calendar member
	MemberInvited  WebhookEventType = "member.invited"
	MemberAccepted WebhookEventType = "member.accepted"
	MemberRejected WebhookEventType = "member.rejected"
	MemberRemoved  WebhookEventType = "member.removed"

	// reminder
	ReminderCreated   WebhookEventType = "reminder.created"
	ReminderUpdated   WebhookEventType = "reminder.updated"
	ReminderDeleted   WebhookEventType = "reminder.deleted"
	ReminderTriggered WebhookEventType = "reminder.triggered"

	// attendee
	AttendeeCreated WebhookEventType = "attendee.created"
	AttendeeUpdated WebhookEventType = "attendee.updated"
	AttendeeDeleted WebhookEventType = "attendee.deleted"
)

// WebhookPayload represents the payload sent to webhook endpoints
type WebhookPayload struct {
	EventType WebhookEventType `json:"event_type"`
	Timestamp int64            `json:"timestamp"`
	Data      interface{}      `json:"data"`
}

// WebhookDispatcher handles queuing webhook delivery jobs
type WebhookDispatcher struct {
	webhookRepo *webhook.Queries
	config      *config.Config
}

// NewWebhookDispatcher creates a new webhook dispatcher
func NewWebhookDispatcher(webhookRepo *webhook.Queries, cfg *config.Config) *WebhookDispatcher {
	return &WebhookDispatcher{
		webhookRepo: webhookRepo,
		config:      cfg,
	}
}

type BatchDeliveryInfo struct {
	BatchSize      int           `json:"batch_size"`
	ChunkIndex     int           `json:"chunk_index"`
	ChunkCount     int           `json:"chunk_count"`
	TotalItems     int           `json:"total_items"`
	ItemsPerChunk  int           `json:"items_per_chunk"`
	ItemsRemaining int           `json:"items_remaining"`
	TotalChunks    int           `json:"total_chunks"`
	Data           []interface{} `json:"data"`
}

func (d *WebhookDispatcher) QueueBatchDelivery(
	ctx context.Context,
	eventType WebhookEventType,
	data []interface{},
) error {
	logger.Info("Queueing batch webhook delivery for event type: %s", eventType)

	batchSize := d.config.WebhookMaxBatchSize

	// split into chunks
	chunks := db.ChunkData(data, batchSize)

	for i, chunk := range chunks {
		batchDeliveryInfo := BatchDeliveryInfo{
			BatchSize:      batchSize,
			ChunkIndex:     i,
			ChunkCount:     len(chunks),
			TotalItems:     len(data),
			ItemsPerChunk:  len(chunk),
			ItemsRemaining: len(data) - len(chunk),
			TotalChunks:    len(chunks),
			Data:           chunk,
		}

		if err := d.QueueDelivery(ctx, eventType, batchDeliveryInfo, nil); err != nil {
			logger.Error("Failed to queue webhook delivery for event type %s: %v", eventType, err)
			return err
		}
	}

	return nil
}

// QueueDelivery queues a webhook delivery for all matching webhooks
func (d *WebhookDispatcher) QueueDelivery(
	ctx context.Context,
	eventType WebhookEventType,
	data interface{},
	eventStartTs *int64,
) error {
	logger.Info("Queueing webhook delivery for event type: %s", eventType)

	// Get all active webhooks for this event type
	webhooks, err := d.webhookRepo.GetActiveWebhooksByEventType(ctx, string(eventType))
	if err != nil {
		logger.Error("Failed to get webhooks for event type %s: %v", eventType, err)
		return fmt.Errorf("failed to get webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		logger.Debug("No active webhooks found for event type: %s", eventType)
		return nil
	}

	logger.Info("Found %d active webhook(s) for event type %s", len(webhooks), eventType)

	// Prepare payload
	payload := WebhookPayload{
		EventType: eventType,
		Timestamp: time.Now().UTC().Unix(),
		Data:      data,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal webhook payload: %v", err)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	logger.Debug("Webhook payload size: %d bytes", len(payloadJSON))

	now := time.Now().UTC().Unix()
	jobsCreated := 0

	// Create a job for each webhook
	for _, wh := range webhooks {
		job := &webhook.WebhookJob{
			JobUID:        uuid.New(),
			WebhookUID:    wh.WebhookUID,
			EventType:     string(eventType),
			Payload:       payloadJSON,
			EventStartTs:  eventStartTs,
			ScheduledAtTs: now,
			NextRetryTs:   now, // Ready for immediate delivery
			AttemptNumber: 0,
			Status:        webhook.JobStatusPending,
			CreatedTs:     now,
		}

		logger.Debug(
			"Creating webhook job %s for webhook %s (URL: %s)",
			job.JobUID,
			wh.WebhookUID,
			wh.URL,
		)

		if err := d.webhookRepo.CreateWebhookJob(ctx, job); err != nil {
			logger.Error(
				"Failed to create webhook job %s for webhook %s: %v",
				job.JobUID,
				wh.WebhookUID,
				err,
			)
			continue
		}

		jobsCreated++
		logger.Info("Created webhook job %s for webhook %s", job.JobUID, wh.WebhookUID)
	}

	logger.Info(
		"Successfully created %d/%d webhook job(s) for event type %s",
		jobsCreated,
		len(webhooks),
		eventType,
	)
	return nil
}

// ============================================================================
// Webhook Delivery Worker
// ============================================================================

// WebhookWorker handles processing and delivering webhook jobs
type WebhookWorker struct {
	webhookRepo *webhook.Queries
	config      *config.Config
	httpClient  *http.Client
	stopCh      chan struct{}
	jobCh       chan *webhook.WebhookJob
	wg          sync.WaitGroup
}

// NewWebhookWorker creates a new webhook delivery worker
func NewWebhookWorker(webhookRepo *webhook.Queries, cfg *config.Config) *WebhookWorker {
	return &WebhookWorker{
		webhookRepo: webhookRepo,
		config:      cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Default timeout, overridden per-webhook
		},
		stopCh: make(chan struct{}),
		jobCh:  make(chan *webhook.WebhookJob, 100), // Buffered channel for job queue
	}
}

// Start begins the webhook delivery worker pool
func (w *WebhookWorker) Start(ctx context.Context) {
	logger.Info(
		"Starting webhook delivery system (workers: %d, poll interval: 1s)",
		w.config.WebhookWorkerConcurrency,
	)

	// Start pre-allocated worker pool
	for i := 0; i < w.config.WebhookWorkerConcurrency; i++ {
		w.wg.Add(1)
		go w.deliveryWorker(ctx, i)
	}
	logger.Info("Started %d delivery worker(s)", w.config.WebhookWorkerConcurrency)

	// Start single poller that fetches jobs from DB
	w.wg.Add(1)
	go w.jobPoller(ctx)

	// Start stale job checker
	w.wg.Add(1)
	go w.staleJobChecker(ctx)

	// Wait for all workers to complete
	w.wg.Wait()
	logger.Info("Webhook delivery worker stopped")
}

// Stop signals the worker to stop
func (w *WebhookWorker) Stop() {
	logger.Info("Stopping webhook delivery system...")
	close(w.stopCh)
	close(w.jobCh)
	w.wg.Wait()
}

// jobPoller is the single goroutine that polls the database for pending jobs
func (w *WebhookWorker) jobPoller(ctx context.Context) {
	defer w.wg.Done()

	logger.Info("Job poller started using interval: 1 second")

	ticker := time.NewTicker(1 * time.Second) // Poll every second
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Job poller stopping (context cancelled)")
			return
		case <-w.stopCh:
			logger.Info("Job poller stopping (stop signal)")
			return
		case <-ticker.C:
			w.fetchAndQueueJobs(ctx)
		}
	}
}

// fetchAndQueueJobs fetches all pending jobs and queues them for workers
func (w *WebhookWorker) fetchAndQueueJobs(ctx context.Context) {
	logger.Debug("Polling database for pending webhook jobs...")

	// Fetch ALL pending jobs (no limit)
	jobs, err := w.webhookRepo.GetPendingWebhookJobs(ctx)
	if err != nil {
		logger.Error("Failed to get pending webhook jobs: %v", err)
		return
	}

	logger.Debug("Database query returned %d job(s)", len(jobs))

	if len(jobs) == 0 {
		return
	}

	logger.Info("Found %d pending job(s), queueing for delivery...", len(jobs))

	// Queue jobs for workers (non-blocking)
	queued := 0
	skipped := 0

	for _, job := range jobs {
		select {
		case w.jobCh <- job:
			queued++
			logger.Debug("Queued job %s to channel", job.JobUID)
		case <-ctx.Done():
			logger.Warn("Context cancelled while queueing jobs, queued %d/%d", queued, len(jobs))
			return
		default:
			// Channel full, log and continue (job will be picked up in next poll)
			skipped++
			logger.Warn("Job queue full, skipping job %s (will retry in next poll)", job.JobUID)
		}
	}

	logger.Info("Queued %d job(s) for delivery (skipped: %d)", queued, skipped)
}

// deliveryWorker is a worker that processes jobs from the queue
func (w *WebhookWorker) deliveryWorker(ctx context.Context, id int) {
	defer w.wg.Done()

	logger.Info("Delivery worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Delivery worker %d stopping (context cancelled)", id)
			return
		case <-w.stopCh:
			logger.Info("Delivery worker %d stopping (stop signal)", id)
			return
		case job, ok := <-w.jobCh:
			if !ok {
				logger.Info("Delivery worker %d stopping (job channel closed)", id)
				return
			}
			logger.Debug("Worker %d picked up job %s", id, job.JobUID)
			w.processJob(ctx, job)
		}
	}
}

// processJob delivers a single webhook job
func (w *WebhookWorker) processJob(ctx context.Context, job *webhook.WebhookJob) {
	logger.Info("Processing webhook job %s (webhook: %s, event: %s, attempt: %d)",
		job.JobUID, job.WebhookUID, job.EventType, job.AttemptNumber)

	// Check if job is stale
	if job.EventStartTs != nil && job.ScheduledAtTs > *job.EventStartTs {
		logger.Warn("Job %s is stale (scheduled at %d after event start %d)",
			job.JobUID, job.ScheduledAtTs, *job.EventStartTs)
		if err := w.webhookRepo.UpdateWebhookJobStatus(ctx, job.JobUID, webhook.JobStatusStale, nil); err != nil {
			logger.Error("Failed to mark job %s as stale: %v", job.JobUID, err)
		}
		return
	}

	// Get the webhook configuration
	wh, err := w.webhookRepo.GetWebhook(ctx, job.WebhookUID)
	if err != nil {
		logger.Error("Failed to get webhook %s for job %s: %v", job.WebhookUID, job.JobUID, err)
		errMsg := err.Error()
		if updateErr := w.webhookRepo.UpdateWebhookJobStatus(ctx, job.JobUID, webhook.JobStatusFailed, &errMsg); updateErr != nil {
			logger.Error("Failed to update job status to failed: %v", updateErr)
		}
		return
	}

	logger.Debug("Delivering webhook to URL: %s (timeout: %ds, max retries: %d)",
		wh.URL, wh.TimeoutSeconds, wh.RetryCount)

	// Deliver the webhook
	success, httpStatus, errMsg := w.deliverWebhook(ctx, wh, job)

	if success {
		logger.Info("✓ Webhook job %s delivered successfully (HTTP %d)", job.JobUID, httpStatus)

		// Mark as completed
		if err := w.webhookRepo.UpdateWebhookJobStatus(ctx, job.JobUID, webhook.JobStatusCompleted, nil); err != nil {
			logger.Error("Failed to mark job %s as completed: %v", job.JobUID, err)
		}

		// Reset webhook failure count
		if err := w.webhookRepo.ResetWebhookFailure(ctx, wh.WebhookUID); err != nil {
			logger.Error("Failed to reset webhook failure count for %s: %v", wh.WebhookUID, err)
		}

		// Record delivery
		w.recordDelivery(ctx, wh, job, httpStatus, nil, job.AttemptNumber)
	} else {
		// Check if we should retry
		job.AttemptNumber++

		if job.AttemptNumber >= wh.RetryCount {
			// Max retries reached - mark as failed
			logger.Error("✗ Webhook job %s failed permanently after %d attempts (HTTP %d): %s",
				job.JobUID, job.AttemptNumber, httpStatus, errMsg)

			if err := w.webhookRepo.UpdateWebhookJobStatus(ctx, job.JobUID, webhook.JobStatusFailed, &errMsg); err != nil {
				logger.Error("Failed to mark job %s as failed: %v", job.JobUID, err)
			}

			// Increment webhook failure count
			if err := w.webhookRepo.IncrementWebhookFailure(ctx, wh.WebhookUID); err != nil {
				logger.Error("Failed to increment webhook failure count for %s: %v", wh.WebhookUID, err)
			}

			// Record failed delivery
			w.recordDelivery(ctx, wh, job, httpStatus, &errMsg, job.AttemptNumber)
		} else {
			// Schedule retry with exponential backoff
			delay := w.calculateBackoff(job.AttemptNumber)
			nextRetryTs := time.Now().UTC().Add(delay).Unix()

			logger.Warn("Retry attempt %d/%d for webhook job at %s URL: %s failed (HTTP %d), scheduling retry in %v: %s",
				job.AttemptNumber, wh.RetryCount, job.JobUID, wh.URL, httpStatus, delay, errMsg)

			if err := w.webhookRepo.UpdateWebhookJobRetry(ctx, job.JobUID, nextRetryTs, job.AttemptNumber, errMsg); err != nil {
				logger.Error("Failed to schedule retry for job %s: %v", job.JobUID, err)
			}
		}
	}
}

// deliverWebhook performs the actual HTTP delivery
func (w *WebhookWorker) deliverWebhook(
	ctx context.Context,
	wh *webhook.Webhook,
	job *webhook.WebhookJob,
) (success bool, httpStatus int, errMsg string) {
	startTime := time.Now()

	logger.Debug("→ Sending webhook request to %s (job: %s)", wh.URL, job.JobUID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, nil)
	if err != nil {
		logger.Error("Failed to create HTTP request for job %s: %v", job.JobUID, err)
		return false, 0, fmt.Sprintf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", job.EventType)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UTC().Unix()))
	req.Header.Set("X-Webhook-Delivery", job.JobUID.String())

	// Calculate HMAC signature
	signature := w.calculateSignature(job.Payload, wh.Secret)
	req.Header.Set("X-Webhook-Signature", signature)

	logger.Debug("Webhook headers: Event=%s, Delivery=%s, Signature=%s...",
		job.EventType, job.JobUID.String(), signature[:16])

	// Set body
	req.Body = io.NopCloser(bytes.NewReader(job.Payload))
	req.ContentLength = int64(len(job.Payload))

	logger.Debug("Request body length: %d bytes", len(job.Payload))

	// Create client with custom timeout
	client := &http.Client{
		Timeout: time.Duration(wh.TimeoutSeconds) * time.Second,
	}

	// Execute request; URL is from stored webhook config, not user input at request time
	// #nosec G704
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("← Webhook request failed for job %s after %v: %v", job.JobUID, duration, err)
		return false, 0, fmt.Sprintf("request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("failed to close response body: %v", closeErr)
		}
	}()

	logger.Debug("← Received response HTTP %d from %s in %v", resp.StatusCode, wh.URL, duration)

	// Check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info(
			"Webhook delivered successfully to %s (HTTP %d) in %v",
			wh.URL,
			resp.StatusCode,
			duration,
		)
		return true, resp.StatusCode, ""
	}

	// Read error response
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	errorMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	logger.Warn("Webhook delivery failed to %s: %s (took %v)", wh.URL, errorMsg, duration)

	return false, resp.StatusCode, errorMsg
}

// calculateSignature computes the HMAC-SHA256 signature for the payload
func (w *WebhookWorker) calculateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// calculateBackoff computes exponential backoff delay
func (w *WebhookWorker) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base_delay * 2^attempt
	// With jitter to avoid thundering herd
	multiplier := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(w.config.WebhookBaseRetryDelay) * multiplier)

	// Cap at 1 hour
	maxDelay := 1 * time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// recordDelivery records a webhook delivery attempt
func (w *WebhookWorker) recordDelivery(
	ctx context.Context,
	wh *webhook.Webhook,
	job *webhook.WebhookJob,
	httpStatus int,
	errMsg *string,
	attemptNumber int,
) {
	delivery := &webhook.WebhookDelivery{
		DeliveryUID:   uuid.New(),
		WebhookUID:    wh.WebhookUID,
		EventType:     job.EventType,
		Payload:       job.Payload,
		HTTPStatus:    &httpStatus,
		ErrorMessage:  errMsg,
		AttemptNumber: attemptNumber,
		DeliveredAtTs: time.Now().UTC().Unix(),
	}

	if err := w.webhookRepo.CreateWebhookDelivery(ctx, delivery); err != nil {
		logger.Error("Failed to record webhook delivery: %v", err)
	}
}

// staleJobChecker periodically marks stale jobs
func (w *WebhookWorker) staleJobChecker(ctx context.Context) {
	logger.Info("Stale job checker started using interval: 5 minutes")
	defer w.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stale job checker stopping (context cancelled)")
			return
		case <-w.stopCh:
			logger.Info("Stale job checker stopping (stop signal)")
			return
		case <-ticker.C:
			logger.Debug("Checking for stale jobs...")
			count, err := w.webhookRepo.MarkStaleJobs(ctx)
			if err != nil {
				logger.Error("Failed to mark stale jobs: %v", err)
			} else if count > 0 {
				logger.Info("Marked %d webhook jobs as stale", count)
			}

			// Cleanup old jobs (older than 7 days)
			sevenDaysAgo := time.Now().UTC().Add(-7 * 24 * time.Hour).Unix()
			deleted, err := w.webhookRepo.CleanupOldJobs(ctx, sevenDaysAgo)
			if err != nil {
				logger.Error("Failed to cleanup old jobs: %v", err)
			} else if deleted > 0 {
				logger.Info("Cleaned up %d old webhook jobs", deleted)
			}

			// update webhook failure counts
			count, err = w.webhookRepo.UpdateWebhookFailureCount(ctx)
			if err != nil {
				logger.Error("Failed to update webhook failure counts: %v", err)
			} else if count > 0 {
				logger.Info("Updated %d webhook failure counts", count)
			}
		}
	}
}
