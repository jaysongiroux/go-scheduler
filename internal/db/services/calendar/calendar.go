package calendar

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// Calendar represents a calendar
type Calendar struct {
	CalendarUID            uuid.UUID       `json:"calendar_uid"`
	AccountID              string          `json:"account_id"`
	CreatedTs              int64           `json:"created_ts"`
	UpdatedTs              int64           `json:"updated_ts"`
	Settings               json.RawMessage `json:"settings"`
	Metadata               json.RawMessage `json:"metadata"`
	IcsURL                 *string         `json:"ics_url,omitempty"`
	IcsAuthType            *string         `json:"ics_auth_type,omitempty"`
	IcsAuthCredentials     []byte          `json:"-"` // Never expose credentials in JSON
	IcsLastSyncTs          *int64          `json:"ics_last_sync_ts,omitempty"`
	IcsLastSyncStatus      *string         `json:"ics_last_sync_status,omitempty"`
	IcsSyncIntervalSeconds *int            `json:"ics_sync_interval_seconds,omitempty"`
	IcsErrorMessage        *string         `json:"ics_error_message,omitempty"`
	IcsLastEtag            *string         `json:"ics_last_etag,omitempty"`
	IcsLastModified        *string         `json:"ics_last_modified,omitempty"`
	IsReadOnly             bool            `json:"is_read_only"`
	SyncOnPartialFailure   bool            `json:"sync_on_partial_failure"`
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateCalendar creates a new calendar
func (q *Queries) CreateCalendar(ctx context.Context, cal *Calendar) error {
	// check if the account exists
	query := `
		INSERT INTO calendars (
			calendar_uid, account_id, created_ts, updated_ts, settings, metadata,
			ics_url, ics_auth_type, ics_auth_credentials, ics_last_sync_ts,
			ics_last_sync_status, ics_sync_interval_seconds, ics_error_message,
			ics_last_etag, ics_last_modified, is_read_only, sync_on_partial_failure
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		cal.CalendarUID,
		cal.AccountID,
		cal.CreatedTs,
		cal.UpdatedTs,
		cal.Settings,
		cal.Metadata,
		cal.IcsURL,
		cal.IcsAuthType,
		cal.IcsAuthCredentials,
		cal.IcsLastSyncTs,
		cal.IcsLastSyncStatus,
		cal.IcsSyncIntervalSeconds,
		cal.IcsErrorMessage,
		cal.IcsLastEtag,
		cal.IcsLastModified,
		cal.IsReadOnly,
		cal.SyncOnPartialFailure,
	)

	return err
}

// GetCalendar retrieves a calendar by ID
func (q *Queries) GetCalendar(ctx context.Context, calendarUID uuid.UUID) (*Calendar, error) {
	query := `
		SELECT 
			calendar_uid, account_id, created_ts, updated_ts, settings, metadata,
			ics_url, ics_auth_type, ics_auth_credentials, ics_last_sync_ts,
			ics_last_sync_status, ics_sync_interval_seconds, ics_error_message,
			ics_last_etag, ics_last_modified, is_read_only, sync_on_partial_failure
		FROM calendars
		WHERE calendar_uid = $1
	`

	var cal Calendar
	err := q.pool.DB().QueryRowContext(ctx, query, calendarUID).Scan(
		&cal.CalendarUID,
		&cal.AccountID,
		&cal.CreatedTs,
		&cal.UpdatedTs,
		&cal.Settings,
		&cal.Metadata,
		&cal.IcsURL,
		&cal.IcsAuthType,
		&cal.IcsAuthCredentials,
		&cal.IcsLastSyncTs,
		&cal.IcsLastSyncStatus,
		&cal.IcsSyncIntervalSeconds,
		&cal.IcsErrorMessage,
		&cal.IcsLastEtag,
		&cal.IcsLastModified,
		&cal.IsReadOnly,
		&cal.SyncOnPartialFailure,
	)

	if err != nil {
		return nil, err
	}

	return &cal, nil
}

// GetUserCalendars retrieves all calendars for a user
func (q *Queries) GetUserCalendars(
	ctx context.Context,
	userID string,
	limit int,
	offset int,
) ([]*Calendar, error) {
	query := `
		SELECT 
			calendar_uid, account_id, created_ts, updated_ts, settings, metadata,
			ics_url, ics_auth_type, ics_auth_credentials, ics_last_sync_ts,
			ics_last_sync_status, ics_sync_interval_seconds, ics_error_message,
			ics_last_etag, ics_last_modified, is_read_only, sync_on_partial_failure
		FROM calendars
		WHERE account_id = $1
		ORDER BY created_ts DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	calendars := make([]*Calendar, 0)
	for rows.Next() {
		var cal Calendar
		err := rows.Scan(
			&cal.CalendarUID,
			&cal.AccountID,
			&cal.CreatedTs,
			&cal.UpdatedTs,
			&cal.Settings,
			&cal.Metadata,
			&cal.IcsURL,
			&cal.IcsAuthType,
			&cal.IcsAuthCredentials,
			&cal.IcsLastSyncTs,
			&cal.IcsLastSyncStatus,
			&cal.IcsSyncIntervalSeconds,
			&cal.IcsErrorMessage,
			&cal.IcsLastEtag,
			&cal.IcsLastModified,
			&cal.IsReadOnly,
			&cal.SyncOnPartialFailure,
		)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, &cal)
	}

	return calendars, nil
}

// UpdateCalendar updates a calendar
func (q *Queries) UpdateCalendar(ctx context.Context, cal *Calendar) error {
	query := `
		UPDATE calendars
		SET
		    updated_ts = $2,
		    settings = $3,
		    metadata = $4,
		    ics_url = $5,
		    ics_auth_type = $6,
		    ics_auth_credentials = $7,
		    ics_last_sync_ts = $8,
		    ics_last_sync_status = $9,
		    ics_sync_interval_seconds = $10,
		    ics_error_message = $11,
		    ics_last_etag = $12,
		    ics_last_modified = $13,
		    is_read_only = $14,
		    sync_on_partial_failure = $15
		WHERE calendar_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		cal.CalendarUID,
		cal.UpdatedTs,
		cal.Settings,
		cal.Metadata,
		cal.IcsURL,
		cal.IcsAuthType,
		cal.IcsAuthCredentials,
		cal.IcsLastSyncTs,
		cal.IcsLastSyncStatus,
		cal.IcsSyncIntervalSeconds,
		cal.IcsErrorMessage,
		cal.IcsLastEtag,
		cal.IcsLastModified,
		cal.IsReadOnly,
		cal.SyncOnPartialFailure,
	)

	return err
}

// DeleteCalendar deletes a calendar
func (q *Queries) DeleteCalendar(ctx context.Context, calendarUID uuid.UUID) error {
	query := `DELETE FROM calendars WHERE calendar_uid = $1`
	_, err := q.pool.DB().ExecContext(ctx, query, calendarUID)
	return err
}

// GetCalendarsNeedingSync retrieves calendars that need ICS synchronization
// Uses FOR UPDATE SKIP LOCKED to prevent concurrent sync attempts
func (q *Queries) GetCalendarsNeedingSync(ctx context.Context, batchSize int) ([]*Calendar, error) {
	query := `
		SELECT 
			calendar_uid, account_id, created_ts, updated_ts, settings, metadata,
			ics_url, ics_auth_type, ics_auth_credentials, ics_last_sync_ts,
			ics_last_sync_status, ics_sync_interval_seconds, ics_error_message,
			ics_last_etag, ics_last_modified, is_read_only, sync_on_partial_failure
		FROM calendars
		WHERE ics_url IS NOT NULL
		AND (
			ics_last_sync_ts IS NULL 
			OR (EXTRACT(EPOCH FROM NOW())::BIGINT - ics_last_sync_ts) >= COALESCE(ics_sync_interval_seconds, 86400)
		)
		ORDER BY COALESCE(ics_last_sync_ts, 0) ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, batchSize)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	calendars := make([]*Calendar, 0)
	for rows.Next() {
		var cal Calendar
		err := rows.Scan(
			&cal.CalendarUID,
			&cal.AccountID,
			&cal.CreatedTs,
			&cal.UpdatedTs,
			&cal.Settings,
			&cal.Metadata,
			&cal.IcsURL,
			&cal.IcsAuthType,
			&cal.IcsAuthCredentials,
			&cal.IcsLastSyncTs,
			&cal.IcsLastSyncStatus,
			&cal.IcsSyncIntervalSeconds,
			&cal.IcsErrorMessage,
			&cal.IcsLastEtag,
			&cal.IcsLastModified,
			&cal.IsReadOnly,
			&cal.SyncOnPartialFailure,
		)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, &cal)
	}

	return calendars, nil
}

// UpdateSyncStatus updates the sync status fields for a calendar
func (q *Queries) UpdateSyncStatus(
	ctx context.Context,
	calendarUID uuid.UUID,
	status string,
	errorMsg *string,
	timestamp int64,
	etag *string,
	lastModified *string,
) error {
	query := `
		UPDATE calendars
		SET 
			ics_last_sync_ts = $2,
			ics_last_sync_status = $3,
			ics_error_message = $4,
			ics_last_etag = $5,
			ics_last_modified = $6,
			updated_ts = $2
		WHERE calendar_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		calendarUID,
		timestamp,
		status,
		errorMsg,
		etag,
		lastModified,
	)

	return err
}
