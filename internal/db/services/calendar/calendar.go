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
	CalendarUID uuid.UUID       `json:"calendar_uid"`
	AccountID   string          `json:"account_id"`
	CreatedTs   int64           `json:"created_ts"`
	UpdatedTs   int64           `json:"updated_ts"`
	Settings    json.RawMessage `json:"settings"`
	Metadata    json.RawMessage `json:"metadata"`
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
		INSERT INTO calendars (calendar_uid, account_id, created_ts, updated_ts, settings, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		cal.CalendarUID,
		cal.AccountID,
		cal.CreatedTs,
		cal.UpdatedTs,
		cal.Settings,
		cal.Metadata,
	)

	return err
}

// GetCalendar retrieves a calendar by ID
func (q *Queries) GetCalendar(ctx context.Context, calendarUID uuid.UUID) (*Calendar, error) {
	query := `
		SELECT calendar_uid, account_id, created_ts, updated_ts, settings, metadata
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
	)

	if err != nil {
		return nil, err
	}

	return &cal, nil
}

// GetUserCalendars retrieves all calendars for a user
func (q *Queries) GetUserCalendars(ctx context.Context, userID string, limit int, offset int) ([]*Calendar, error) {
	query := `
		SELECT calendar_uid, account_id, created_ts, updated_ts, settings, metadata
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
		    metadata = $4
		WHERE calendar_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		cal.CalendarUID,
		cal.UpdatedTs,
		cal.Settings,
		cal.Metadata,
	)

	return err
}

// DeleteCalendar deletes a calendar
func (q *Queries) DeleteCalendar(ctx context.Context, calendarUID uuid.UUID) error {
	query := `DELETE FROM calendars WHERE calendar_uid = $1`
	_, err := q.pool.DB().ExecContext(ctx, query, calendarUID)
	return err
}
