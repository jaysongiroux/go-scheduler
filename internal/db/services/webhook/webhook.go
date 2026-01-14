package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/lib/pq"
)

// Webhook represents a webhook configuration
type Webhook struct {
	WebhookUID        uuid.UUID      `json:"webhook_uid"`
	URL               string         `json:"url"`
	// #nosec G117 -- webhook signing secret by design
	Secret            string         `json:"secret"`
	EventTypes        pq.StringArray `json:"event_types"`
	IsActive          bool           `json:"is_active"`
	RetryCount        int            `json:"retry_count"`
	TimeoutSeconds    int            `json:"timeout_seconds"`
	LastTriggeredAtTs *int64         `json:"last_triggered_at_ts,omitempty"`
	LastSuccessAtTs   *int64         `json:"last_success_at_ts,omitempty"`
	LastFailureAtTs   *int64         `json:"last_failure_at_ts,omitempty"`
	FailureCount      int            `json:"failure_count"`
	CreatedTs         int64          `json:"created_ts"`
	UpdatedTs         int64          `json:"updated_ts"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	DeliveryUID    uuid.UUID       `json:"delivery_uid"`
	WebhookUID     uuid.UUID       `json:"webhook_uid"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	HTTPStatus     *int            `json:"http_status,omitempty"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	ResponseBody   *string         `json:"response_body,omitempty"`
	ResponseTimeMs *int            `json:"response_time_ms,omitempty"`
	AttemptNumber  int             `json:"attempt_number"`
	DeliveredAtTs  int64           `json:"delivered_at_ts"`
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateWebhook creates a new webhook
func (q *Queries) CreateWebhook(ctx context.Context, w *Webhook) error {
	query := `
		INSERT INTO webhooks (
			webhook_uid, url, secret, event_types,
			is_active, retry_count, timeout_seconds, created_ts, updated_ts
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	// Convert []string to interface{} for pq.Array() which supports TEXT[]
	eventTypes := pq.Array(w.EventTypes)

	_, err := q.pool.DB().ExecContext(ctx, query,
		w.WebhookUID,
		w.URL,
		w.Secret,
		eventTypes,
		w.IsActive,
		w.RetryCount,
		w.TimeoutSeconds,
		w.CreatedTs,
		w.UpdatedTs,
	)

	return err
}

// GetWebhook retrieves a webhook by ID
func (q *Queries) GetWebhook(ctx context.Context, webhookUID uuid.UUID) (*Webhook, error) {
	query := `
		SELECT 
			webhook_uid, url, secret, event_types,
			is_active, retry_count, timeout_seconds,
			last_triggered_at_ts, last_success_at_ts, last_failure_at_ts,
			failure_count, created_ts, updated_ts
		FROM webhooks
		WHERE webhook_uid = $1
	`

	var w Webhook
	err := q.pool.DB().QueryRowContext(ctx, query, webhookUID).Scan(
		&w.WebhookUID,
		&w.URL,
		&w.Secret,
		&w.EventTypes,
		&w.IsActive,
		&w.RetryCount,
		&w.TimeoutSeconds,
		&w.LastTriggeredAtTs,
		&w.LastSuccessAtTs,
		&w.LastFailureAtTs,
		&w.FailureCount,
		&w.CreatedTs,
		&w.UpdatedTs,
	)

	if err != nil {
		return nil, err
	}

	return &w, nil
}

// returns a paginated list of webhook endpoints
func (q *Queries) GetWebhookEndpoints(
	ctx context.Context,
	offset int,
	limit int,
) ([]*Webhook, error) {
	query := `
		SELECT 
			webhook_uid, url, secret, event_types,
			is_active, retry_count, timeout_seconds,
			last_triggered_at_ts, last_success_at_ts, last_failure_at_ts,
			failure_count, created_ts, updated_ts
		FROM webhooks
		ORDER BY created_ts DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	webhooks := make([]*Webhook, 0)
	for rows.Next() {
		var (
			w Webhook
		)
		err := rows.Scan(
			&w.WebhookUID,
			&w.URL,
			&w.Secret,
			&w.EventTypes,
			&w.IsActive,
			&w.RetryCount,
			&w.TimeoutSeconds,
			&w.LastTriggeredAtTs,
			&w.LastSuccessAtTs,
			&w.LastFailureAtTs,
			&w.FailureCount,
			&w.CreatedTs,
			&w.UpdatedTs,
		)
		if err != nil {
			return nil, err
		}

		webhooks = append(webhooks, &w)
	}
	return webhooks, nil
}

// GetActiveWebhooksByEventType retrieves active webhooks for an account filtered by event type
func (q *Queries) GetActiveWebhooksByEventType(
	ctx context.Context,
	eventType string,
) ([]*Webhook, error) {
	query := `
		SELECT 
			webhook_uid, url, secret, event_types,
			is_active, retry_count, timeout_seconds,
			last_triggered_at_ts, last_success_at_ts, last_failure_at_ts,
			failure_count, created_ts, updated_ts
		FROM webhooks
		WHERE is_active = true
		  AND $1 = ANY(event_types)
		ORDER BY created_ts DESC
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, eventType)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	webhooks := make([]*Webhook, 0)
	for rows.Next() {
		var w Webhook
		err := rows.Scan(
			&w.WebhookUID,
			&w.URL,
			&w.Secret,
			&w.EventTypes,
			&w.IsActive,
			&w.RetryCount,
			&w.TimeoutSeconds,
			&w.LastTriggeredAtTs,
			&w.LastSuccessAtTs,
			&w.LastFailureAtTs,
			&w.FailureCount,
			&w.CreatedTs,
			&w.UpdatedTs,
		)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, &w)
	}

	return webhooks, nil
}

// UpdateWebhook updates a webhook
func (q *Queries) UpdateWebhook(ctx context.Context, w *Webhook) error {
	query := `
		UPDATE webhooks
		SET url = $2,
		    event_types = $3,
		    is_active = $4,
		    retry_count = $5,
		    timeout_seconds = $6
		WHERE webhook_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		w.WebhookUID,
		w.URL,
		w.EventTypes,
		w.IsActive,
		w.RetryCount,
		w.TimeoutSeconds,
	)

	return err
}

// DeleteWebhook deletes a webhook
func (q *Queries) DeleteWebhook(ctx context.Context, webhookUID uuid.UUID) error {
	query := `DELETE FROM webhooks WHERE webhook_uid = $1`
	_, err := q.pool.DB().ExecContext(ctx, query, webhookUID)
	return err
}

// IncrementWebhookFailure calls the database function to increment failure count
func (q *Queries) IncrementWebhookFailure(ctx context.Context, webhookUID uuid.UUID) error {
	query := `SELECT increment_webhook_failure($1)`
	_, err := q.pool.DB().ExecContext(ctx, query, webhookUID)
	return err
}

// ResetWebhookFailure calls the database function to reset failure count
func (q *Queries) ResetWebhookFailure(ctx context.Context, webhookUID uuid.UUID) error {
	query := `SELECT reset_webhook_failure($1)`
	_, err := q.pool.DB().ExecContext(ctx, query, webhookUID)
	return err
}

// CreateWebhookDelivery records a webhook delivery attempt
func (q *Queries) CreateWebhookDelivery(ctx context.Context, d *WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			delivery_uid, webhook_uid, event_type, payload,
			http_status, error_message, response_body, response_time_ms,
			attempt_number, delivered_at_ts
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		d.DeliveryUID,
		d.WebhookUID,
		d.EventType,
		d.Payload,
		d.HTTPStatus,
		d.ErrorMessage,
		d.ResponseBody,
		d.ResponseTimeMs,
		d.AttemptNumber,
		d.DeliveredAtTs,
	)

	return err
}

// GetWebhookDeliveries retrieves recent deliveries for a webhook
func (q *Queries) GetWebhookDeliveries(
	ctx context.Context,
	webhookUID uuid.UUID,
	limit int,
	offset int,
) ([]*WebhookDelivery, error) {
	query := `
		SELECT 
			delivery_uid, webhook_uid, event_type, payload,
			http_status, error_message, response_body, response_time_ms,
			attempt_number, delivered_at_ts
		FROM webhook_deliveries
		WHERE webhook_uid = $1
		ORDER BY delivered_at_ts DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, webhookUID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	deliveries := make([]*WebhookDelivery, 0)
	for rows.Next() {
		var d WebhookDelivery
		err := rows.Scan(
			&d.DeliveryUID,
			&d.WebhookUID,
			&d.EventType,
			&d.Payload,
			&d.HTTPStatus,
			&d.ErrorMessage,
			&d.ResponseBody,
			&d.ResponseTimeMs,
			&d.AttemptNumber,
			&d.DeliveredAtTs,
		)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, &d)
	}

	return deliveries, nil
}

// ============================================================================
// Webhook Job Queue (for durable delivery)
// ============================================================================

// WebhookJobStatus represents the status of a webhook delivery job
type WebhookJobStatus string

const (
	JobStatusPending   WebhookJobStatus = "pending"
	JobStatusCompleted WebhookJobStatus = "completed"
	JobStatusFailed    WebhookJobStatus = "failed"
	JobStatusStale     WebhookJobStatus = "stale"
)

// WebhookJob represents a pending webhook delivery job
type WebhookJob struct {
	JobUID        uuid.UUID        `json:"job_uid"`
	WebhookUID    uuid.UUID        `json:"webhook_uid"`
	EventType     string           `json:"event_type"`
	Payload       json.RawMessage  `json:"payload"`
	EventStartTs  *int64           `json:"event_start_ts,omitempty"` // For staleness check
	ScheduledAtTs int64            `json:"scheduled_at_ts"`
	NextRetryTs   int64            `json:"next_retry_ts"`
	AttemptNumber int              `json:"attempt_number"`
	Status        WebhookJobStatus `json:"status"`
	ErrorMessage  *string          `json:"error_message,omitempty"`
	CreatedTs     int64            `json:"created_ts"`
}

// CreateWebhookJob creates a new webhook delivery job
func (q *Queries) CreateWebhookJob(ctx context.Context, job *WebhookJob) error {
	query := `
		INSERT INTO webhook_jobs (
			job_uid, webhook_uid, event_type, payload, event_start_ts,
			scheduled_at_ts, next_retry_ts, attempt_number, status, created_ts
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	var eventStartTs interface{} = nil
	if job.EventStartTs != nil {
		eventStartTs = *job.EventStartTs
	}

	_, err := q.pool.DB().ExecContext(ctx, query,
		job.JobUID,
		job.WebhookUID,
		job.EventType,
		job.Payload,
		eventStartTs,
		job.ScheduledAtTs,
		job.NextRetryTs,
		job.AttemptNumber,
		string(job.Status),
		job.CreatedTs,
	)

	return err
}

// GetPendingWebhookJobs retrieves pending jobs ready for delivery
// If limit is 0, returns all pending jobs
func (q *Queries) GetPendingWebhookJobs(ctx context.Context) ([]*WebhookJob, error) {
	var query string
	now := currentTimestamp()

	logger.Debug("Querying webhook_jobs table (current_ts: %d)", now)

	query = `
		SELECT 
			job_uid, webhook_uid, event_type, payload, event_start_ts,
			scheduled_at_ts, next_retry_ts, attempt_number, status, error_message, created_ts
		FROM webhook_jobs
		WHERE status = 'pending'
		AND next_retry_ts <= $1
		ORDER BY next_retry_ts ASC
		FOR UPDATE SKIP LOCKED
	`

	var rows *sql.Rows
	var err error
	rows, err = q.pool.DB().QueryContext(ctx, query, now)

	if err != nil {
		logger.Error("Database query failed: %v", err)
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	jobs := make([]*WebhookJob, 0)
	rowCount := 0

	for rows.Next() {
		rowCount++
		var job WebhookJob
		var eventStartTs *int64
		var errorMessage *string
		err := rows.Scan(
			&job.JobUID,
			&job.WebhookUID,
			&job.EventType,
			&job.Payload,
			&eventStartTs,
			&job.ScheduledAtTs,
			&job.NextRetryTs,
			&job.AttemptNumber,
			&job.Status,
			&errorMessage,
			&job.CreatedTs,
		)
		if err != nil {
			logger.Error("Failed to scan webhook job row: %v", err)
			return nil, err
		}
		if eventStartTs != nil {
			job.EventStartTs = eventStartTs
		}
		if errorMessage != nil {
			job.ErrorMessage = errorMessage
		}

		logger.Debug(
			"Scanned job: %s (status: %s, next_retry: %d)",
			job.JobUID,
			job.Status,
			job.NextRetryTs,
		)
		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		logger.Error("Error iterating webhook job rows: %v", err)
		return nil, err
	}

	logger.Debug("Query completed: found %d pending job(s)", rowCount)
	return jobs, nil
}

// UpdateWebhookJobStatus updates the status of a webhook job
func (q *Queries) UpdateWebhookJobStatus(
	ctx context.Context,
	jobUID uuid.UUID,
	status WebhookJobStatus,
	errorMessage *string,
) error {
	query := `
		UPDATE webhook_jobs 
		SET status = $2, error_message = $3
		WHERE job_uid = $1
	`
	_, err := q.pool.DB().ExecContext(ctx, query, jobUID, string(status), errorMessage)
	return err
}

// UpdateWebhookJobRetry updates a job for retry with exponential backoff
func (q *Queries) UpdateWebhookJobRetry(
	ctx context.Context,
	jobUID uuid.UUID,
	nextRetryTs int64,
	attemptNumber int,
	errorMessage string,
) error {
	query := `
		UPDATE webhook_jobs 
		SET next_retry_ts = $2, attempt_number = $3, error_message = $4
		WHERE job_uid = $1
	`
	_, err := q.pool.DB().ExecContext(ctx, query, jobUID, nextRetryTs, attemptNumber, errorMessage)
	return err
}

// MarkStaleJobs marks jobs as stale if their scheduled time is past the event start time
func (q *Queries) MarkStaleJobs(ctx context.Context) (int64, error) {
	query := `
		UPDATE webhook_jobs 
		SET status = 'stale'
		WHERE status = 'pending'
		AND event_start_ts IS NOT NULL
		AND scheduled_at_ts > event_start_ts
	`
	result, err := q.pool.DB().ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupOldJobs removes completed/failed/stale jobs older than the specified timestamp
func (q *Queries) CleanupOldJobs(ctx context.Context, olderThanTs int64) (int64, error) {
	query := `
		DELETE FROM webhook_jobs 
		WHERE status IN ('completed', 'failed', 'stale')
		AND created_ts < $1
	`
	result, err := q.pool.DB().ExecContext(ctx, query, olderThanTs)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// updates each webhook's failure count
func (q *Queries) UpdateWebhookFailureCount(ctx context.Context) (int64, error) {
	query := `
			UPDATE webhooks
			SET failure_count = (
				SELECT COUNT(*) FROM webhook_jobs WHERE webhook_uid = webhooks.webhook_uid AND status = 'failed'
			)
	`
	result, err := q.pool.DB().ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetWebhookJob retrieves a single webhook job by ID
func (q *Queries) GetWebhookJob(ctx context.Context, jobUID uuid.UUID) (*WebhookJob, error) {
	query := `
		SELECT 
			job_uid, webhook_uid, event_type, payload, event_start_ts,
			scheduled_at_ts, next_retry_ts, attempt_number, status, error_message, created_ts
		FROM webhook_jobs
		WHERE job_uid = $1
	`

	var job WebhookJob
	var eventStartTs *int64
	var errorMessage *string

	err := q.pool.DB().QueryRowContext(ctx, query, jobUID).Scan(
		&job.JobUID,
		&job.WebhookUID,
		&job.EventType,
		&job.Payload,
		&eventStartTs,
		&job.ScheduledAtTs,
		&job.NextRetryTs,
		&job.AttemptNumber,
		&job.Status,
		&errorMessage,
		&job.CreatedTs,
	)
	if err != nil {
		return nil, err
	}

	job.EventStartTs = eventStartTs
	job.ErrorMessage = errorMessage

	return &job, nil
}

// currentTimestamp returns the current Unix timestamp
func currentTimestamp() int64 {
	return time.Now().UTC().Unix()
}
