package attendee

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// Role constants
const (
	RoleOrganizer = "organizer"
	RoleAttendee  = "attendee"
)

// RSVP status constants
const (
	RSVPPending   = "pending"
	RSVPAccepted  = "accepted"
	RSVPDeclined  = "declined"
	RSVPTentative = "tentative"
)

// Attendee represents an event attendee
type Attendee struct {
	AttendeeUID     uuid.UUID       `json:"attendee_uid"`
	EventUID        uuid.UUID       `json:"event_uid"`
	AccountID       string          `json:"account_id"`
	MasterEventUID  *uuid.UUID      `json:"master_event_uid,omitempty"`
	AttendeeGroupID *uuid.UUID      `json:"attendee_group_id,omitempty"`
	Role            string          `json:"role"`
	RSVPStatus      string          `json:"rsvp_status"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedTs       int64           `json:"created_ts"`
	UpdatedTs       int64           `json:"updated_ts"`
	Archived        bool            `json:"archived"`
	ArchivedTs      *int64          `json:"archived_ts,omitempty"`
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateSingleAttendee adds an attendee to a single event
func (q *Queries) CreateSingleAttendee(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID, role string,
	metadata json.RawMessage,
) (*Attendee, error) {
	// metadata is NOT NULL in DB, use empty object if nil or empty
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id, 
			role, rsvp_status, metadata, archived
		)
		SELECT 
			$1, $2, ce.master_event_uid, NULL, 
			$3, $4, $5, FALSE
		FROM calendar_events ce
		WHERE ce.event_uid = $1
		ON CONFLICT (event_uid, account_id) 
		DO UPDATE SET 
			archived = FALSE,
			archived_ts = NULL,
			role = EXCLUDED.role,
			metadata = EXCLUDED.metadata,
			updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		RETURNING 
			attendee_uid, event_uid, account_id, master_event_uid, 
			attendee_group_id, role, rsvp_status, metadata, 
			created_ts, updated_ts, archived, archived_ts
	`

	rsvpStatus := RSVPPending
	if role == RoleOrganizer {
		rsvpStatus = RSVPAccepted
	}

	var attendee Attendee
	err := q.pool.DB().
		QueryRowContext(ctx, query, eventUID, accountID, role, rsvpStatus, metadata).
		Scan(
			&attendee.AttendeeUID,
			&attendee.EventUID,
			&attendee.AccountID,
			&attendee.MasterEventUID,
			&attendee.AttendeeGroupID,
			&attendee.Role,
			&attendee.RSVPStatus,
			&attendee.Metadata,
			&attendee.CreatedTs,
			&attendee.UpdatedTs,
			&attendee.Archived,
			&attendee.ArchivedTs,
		)
	if err != nil {
		return nil, fmt.Errorf("failed to create attendee: %w", err)
	}

	return &attendee, nil
}

// CreateSeriesAttendee adds an attendee to all events in a series
func (q *Queries) CreateSeriesAttendee(
	ctx context.Context,
	triggerEventUID uuid.UUID,
	accountID, role string,
	metadata json.RawMessage,
) (uuid.UUID, int, error) {
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	rsvpStatus := RSVPPending
	if role == RoleOrganizer {
		rsvpStatus = RSVPAccepted
	}

	groupID := uuid.New()

	query := `
		WITH trigger_info AS (
			SELECT 
				COALESCE(ce.master_event_uid, ce.event_uid) as effective_master_uid,
				ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		),
		affected_events AS (
			SELECT ce.event_uid, ce.master_event_uid
			FROM calendar_events ce, trigger_info ti
			WHERE 
				-- Include master event
				(ce.event_uid = ti.effective_master_uid)
				OR
				-- Include future instances
				(ce.master_event_uid = ti.effective_master_uid 
				 AND ce.start_ts >= ti.trigger_start_ts
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
		)
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id,
			role, rsvp_status, metadata, archived
		)
		SELECT 
			ae.event_uid, $2, ae.master_event_uid, $3,
			$4, $5, $6, FALSE
		FROM affected_events ae
		ON CONFLICT (event_uid, account_id)
		DO UPDATE SET
			archived = FALSE,
			archived_ts = NULL,
			attendee_group_id = EXCLUDED.attendee_group_id,
			role = EXCLUDED.role,
			metadata = EXCLUDED.metadata,
			updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		RETURNING attendee_uid
	`

	rows, err := q.pool.DB().
		QueryContext(ctx, query, triggerEventUID, accountID, groupID, role, rsvpStatus, metadata)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("failed to create series attendees: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		count++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return uuid.Nil, 0, fmt.Errorf("failed to scan attendee uid: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return uuid.Nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	return groupID, count, nil
}

// GetEventAttendees retrieves all attendees for an event
func (q *Queries) GetEventAttendees(
	ctx context.Context,
	eventUID uuid.UUID,
	roleFilter, rsvpFilter string,
) ([]Attendee, error) {
	query := `
		SELECT 
			attendee_uid, event_uid, account_id, master_event_uid,
			attendee_group_id, role, rsvp_status, metadata,
			created_ts, updated_ts, archived, archived_ts
		FROM event_attendees
		WHERE event_uid = $1 
			AND archived = FALSE
			AND ($2::TEXT IS NULL OR role = $2)
			AND ($3::TEXT IS NULL OR rsvp_status = $3)
		ORDER BY 
			CASE WHEN role = 'organizer' THEN 0 ELSE 1 END,
			created_ts ASC
	`

	var roleParam, rsvpParam *string
	if roleFilter != "" {
		roleParam = &roleFilter
	}
	if rsvpFilter != "" {
		rsvpParam = &rsvpFilter
	}

	rows, err := q.pool.DB().QueryContext(ctx, query, eventUID, roleParam, rsvpParam)
	if err != nil {
		return nil, fmt.Errorf("failed to query attendees: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	var attendees []Attendee
	for rows.Next() {
		var a Attendee
		err := rows.Scan(
			&a.AttendeeUID,
			&a.EventUID,
			&a.AccountID,
			&a.MasterEventUID,
			&a.AttendeeGroupID,
			&a.Role,
			&a.RSVPStatus,
			&a.Metadata,
			&a.CreatedTs,
			&a.UpdatedTs,
			&a.Archived,
			&a.ArchivedTs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attendee: %w", err)
		}
		attendees = append(attendees, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return attendees, nil
}

// GetAttendee retrieves a specific attendee by event and account
func (q *Queries) GetAttendee(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID string,
) (*Attendee, error) {
	query := `
		SELECT 
			attendee_uid, event_uid, account_id, master_event_uid,
			attendee_group_id, role, rsvp_status, metadata,
			created_ts, updated_ts, archived, archived_ts
		FROM event_attendees
		WHERE event_uid = $1 AND account_id = $2 AND archived = FALSE
	`

	var attendee Attendee
	err := q.pool.DB().QueryRowContext(ctx, query, eventUID, accountID).Scan(
		&attendee.AttendeeUID,
		&attendee.EventUID,
		&attendee.AccountID,
		&attendee.MasterEventUID,
		&attendee.AttendeeGroupID,
		&attendee.Role,
		&attendee.RSVPStatus,
		&attendee.Metadata,
		&attendee.CreatedTs,
		&attendee.UpdatedTs,
		&attendee.Archived,
		&attendee.ArchivedTs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("attendee not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attendee: %w", err)
	}

	return &attendee, nil
}

// EventWithAttendee represents an event with attendee metadata
type EventWithAttendee struct {
	EventUID       uuid.UUID       `json:"event_uid"`
	CalendarUID    uuid.UUID       `json:"calendar_uid"`
	StartTs        int64           `json:"start_ts"`
	EndTs          int64           `json:"end_ts"`
	Metadata       json.RawMessage `json:"metadata"`
	AttendeeUID    uuid.UUID       `json:"attendee_uid"`
	Role           string          `json:"role"`
	RSVPStatus     string          `json:"rsvp_status"`
	IsCancelled    bool            `json:"is_cancelled"`
	MasterEventUID *uuid.UUID      `json:"master_event_uid,omitempty"`
}

// GetAccountEvents retrieves events where account is an attendee
func (q *Queries) GetAccountEvents(
	ctx context.Context,
	accountID string,
	startTs, endTs int64,
	roleFilter, rsvpFilter string,
) ([]EventWithAttendee, error) {
	query := `
		SELECT 
			ce.event_uid, ce.calendar_uid, ce.start_ts, ce.end_ts, 
			ce.metadata, ce.is_cancelled, ce.master_event_uid,
			ea.attendee_uid, ea.role, ea.rsvp_status
		FROM event_attendees ea
		JOIN calendar_events ce ON ea.event_uid = ce.event_uid
		WHERE ea.account_id = $1
			AND ea.archived = FALSE
			AND ce.is_cancelled = FALSE
			AND ce.start_ts >= $2
			AND ce.end_ts <= $3
			AND ($4::TEXT IS NULL OR ea.role = $4)
			AND ($5::TEXT IS NULL OR ea.rsvp_status = $5)
		ORDER BY ce.start_ts ASC
	`

	var roleParam, rsvpParam *string
	if roleFilter != "" {
		roleParam = &roleFilter
	}
	if rsvpFilter != "" {
		rsvpParam = &rsvpFilter
	}

	rows, err := q.pool.DB().
		QueryContext(ctx, query, accountID, startTs, endTs, roleParam, rsvpParam)
	if err != nil {
		return nil, fmt.Errorf("failed to query account events: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	var events []EventWithAttendee
	for rows.Next() {
		var e EventWithAttendee
		err := rows.Scan(
			&e.EventUID,
			&e.CalendarUID,
			&e.StartTs,
			&e.EndTs,
			&e.Metadata,
			&e.IsCancelled,
			&e.MasterEventUID,
			&e.AttendeeUID,
			&e.Role,
			&e.RSVPStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// CountOrganizers counts the number of organizers for an event
func (q *Queries) CountOrganizers(ctx context.Context, eventUID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM event_attendees
		WHERE event_uid = $1 AND role = 'organizer' AND archived = FALSE
	`

	var count int
	err := q.pool.DB().QueryRowContext(ctx, query, eventUID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count organizers: %w", err)
	}

	return count, nil
}

// IsAccountAttendee checks if an account is an attendee of an event
func (q *Queries) IsAccountAttendee(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID string,
) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM event_attendees
			WHERE event_uid = $1 AND account_id = $2 AND archived = FALSE
		)
	`

	var exists bool
	err := q.pool.DB().QueryRowContext(ctx, query, eventUID, accountID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check attendee: %w", err)
	}

	return exists, nil
}

// UpdateSingleAttendeeRSVP updates RSVP status for a single event
func (q *Queries) UpdateSingleAttendeeRSVP(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID, rsvpStatus string,
) (*Attendee, error) {
	// First validate event hasn't ended
	checkQuery := `
		SELECT start_ts FROM calendar_events WHERE event_uid = $1
	`
	var startTs int64
	err := q.pool.DB().QueryRowContext(ctx, checkQuery, eventUID).Scan(&startTs)
	if err != nil {
		return nil, fmt.Errorf("failed to check event time: %w", err)
	}

	now := time.Now().Unix()
	if startTs < now {
		return nil, fmt.Errorf("cannot update RSVP for events that have already ended")
	}

	query := `
		UPDATE event_attendees
		SET rsvp_status = $1, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $2 AND account_id = $3 AND archived = FALSE
		RETURNING 
			attendee_uid, event_uid, account_id, master_event_uid,
			attendee_group_id, role, rsvp_status, metadata,
			created_ts, updated_ts, archived, archived_ts
	`

	var attendee Attendee
	err = q.pool.DB().QueryRowContext(ctx, query, rsvpStatus, eventUID, accountID).Scan(
		&attendee.AttendeeUID,
		&attendee.EventUID,
		&attendee.AccountID,
		&attendee.MasterEventUID,
		&attendee.AttendeeGroupID,
		&attendee.Role,
		&attendee.RSVPStatus,
		&attendee.Metadata,
		&attendee.CreatedTs,
		&attendee.UpdatedTs,
		&attendee.Archived,
		&attendee.ArchivedTs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("attendee not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update RSVP: %w", err)
	}

	return &attendee, nil
}

// UpdateSeriesAttendeeRSVP updates RSVP status for all events in a series
func (q *Queries) UpdateSeriesAttendeeRSVP(
	ctx context.Context,
	triggerEventUID uuid.UUID,
	accountID, rsvpStatus string,
) (int, error) {
	// First validate trigger event hasn't ended and get group ID
	checkQuery := `
		SELECT ce.start_ts, ea.attendee_group_id
		FROM calendar_events ce
		JOIN event_attendees ea ON ce.event_uid = ea.event_uid
		WHERE ce.event_uid = $1 AND ea.account_id = $2 AND ea.archived = FALSE
	`
	var startTs int64
	var groupID *uuid.UUID
	err := q.pool.DB().
		QueryRowContext(ctx, checkQuery, triggerEventUID, accountID).
		Scan(&startTs, &groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("attendee not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to check event: %w", err)
	}

	now := time.Now().Unix()
	if startTs < now {
		return 0, fmt.Errorf("cannot update RSVP for events that have already ended")
	}

	if groupID == nil {
		return 0, fmt.Errorf("attendee is not part of a series")
	}

	query := `
		WITH trigger_info AS (
			SELECT ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE event_attendees ea
		SET rsvp_status = $2, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE ea.event_uid = ce.event_uid
			AND ea.account_id = $3
			AND ea.attendee_group_id = $4
			AND ea.archived = FALSE
			AND (
				-- Update master event
				ce.event_uid = ce.master_event_uid
				OR
				-- Update future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
		RETURNING ea.attendee_uid
	`

	rows, err := q.pool.DB().
		QueryContext(ctx, query, triggerEventUID, rsvpStatus, accountID, groupID)
	if err != nil {
		return 0, fmt.Errorf("failed to update series RSVP: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		count++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return 0, fmt.Errorf("failed to scan uid: %w", err)
		}
	}

	return count, rows.Err()
}

// UpdateSingleAttendee updates role and/or metadata for a single event attendee
func (q *Queries) UpdateSingleAttendee(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID, role string,
	metadata json.RawMessage,
) (*Attendee, error) {
	// Get current attendee to check if role is changing from organizer
	current, err := q.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		return nil, err
	}

	// If changing from organizer to attendee, validate count
	if current.Role == RoleOrganizer && role == RoleAttendee {
		count, err := q.CountOrganizers(ctx, eventUID)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, fmt.Errorf("cannot remove last organizer from event")
		}
	}

	query := `
		UPDATE event_attendees
		SET role = $1, metadata = $2, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $3 AND account_id = $4 AND archived = FALSE
		RETURNING 
			attendee_uid, event_uid, account_id, master_event_uid,
			attendee_group_id, role, rsvp_status, metadata,
			created_ts, updated_ts, archived, archived_ts
	`

	var attendee Attendee
	err = q.pool.DB().QueryRowContext(ctx, query, role, metadata, eventUID, accountID).Scan(
		&attendee.AttendeeUID,
		&attendee.EventUID,
		&attendee.AccountID,
		&attendee.MasterEventUID,
		&attendee.AttendeeGroupID,
		&attendee.Role,
		&attendee.RSVPStatus,
		&attendee.Metadata,
		&attendee.CreatedTs,
		&attendee.UpdatedTs,
		&attendee.Archived,
		&attendee.ArchivedTs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update attendee: %w", err)
	}

	return &attendee, nil
}

// UpdateSeriesAttendee updates role and/or metadata for all events in a series
func (q *Queries) UpdateSeriesAttendee(
	ctx context.Context,
	triggerEventUID uuid.UUID,
	accountID, role string,
	metadata json.RawMessage,
) (int, error) {
	// Get current attendee to check if role is changing from organizer
	current, err := q.GetAttendee(ctx, triggerEventUID, accountID)
	if err != nil {
		return 0, err
	}

	if current.AttendeeGroupID == nil {
		return 0, fmt.Errorf("attendee is not part of a series")
	}

	// If changing from organizer to attendee, validate count on master
	if current.Role == RoleOrganizer && role == RoleAttendee {
		// Find master event UID
		var masterUID uuid.UUID
		if current.MasterEventUID != nil {
			masterUID = *current.MasterEventUID
		} else {
			masterUID = triggerEventUID
		}

		count, err := q.CountOrganizers(ctx, masterUID)
		if err != nil {
			return 0, err
		}
		if count <= 1 {
			return 0, fmt.Errorf("cannot remove last organizer from event series")
		}
	}

	query := `
		WITH trigger_info AS (
			SELECT ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE event_attendees ea
		SET role = $2, metadata = $3, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE ea.event_uid = ce.event_uid
			AND ea.account_id = $4
			AND ea.attendee_group_id = $5
			AND ea.archived = FALSE
			AND (
				-- Update master event
				ce.event_uid = ce.master_event_uid
				OR
				-- Update future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
		RETURNING ea.attendee_uid
	`

	rows, err := q.pool.DB().
		QueryContext(ctx, query, triggerEventUID, role, metadata, accountID, current.AttendeeGroupID)
	if err != nil {
		return 0, fmt.Errorf("failed to update series attendee: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		count++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return 0, fmt.Errorf("failed to scan uid: %w", err)
		}
	}

	return count, rows.Err()
}

// DeleteSingleAttendee soft-deletes an attendee from a single event
func (q *Queries) DeleteSingleAttendee(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID string,
) (int, error) {
	// Get current attendee
	current, err := q.GetAttendee(ctx, eventUID, accountID)
	if err != nil {
		return 0, err
	}

	// If removing organizer, validate count
	if current.Role == RoleOrganizer {
		count, err := q.CountOrganizers(ctx, eventUID)
		if err != nil {
			return 0, err
		}
		if count <= 1 {
			return 0, fmt.Errorf("cannot remove last organizer from event")
		}
	}

	tx, err := q.pool.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil &&
			!errors.Is(rollbackErr, sql.ErrTxDone) {
			logger.Error("failed to rollback transaction: %v", rollbackErr)
		}
	}()

	// Soft delete attendee
	deleteQuery := `
		UPDATE event_attendees
		SET archived = TRUE, archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $1 AND account_id = $2 AND archived = FALSE
	`
	_, err = tx.ExecContext(ctx, deleteQuery, eventUID, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete attendee: %w", err)
	}

	// Soft delete their reminders
	reminderQuery := `
		UPDATE calendar_event_reminders
		SET archived = TRUE, archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $1 AND account_id = $2 AND archived = FALSE
	`
	result, err := tx.ExecContext(ctx, reminderQuery, eventUID, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete reminders: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	remindersDeleted, _ := result.RowsAffected()
	return int(remindersDeleted), nil
}

// DeleteSeriesAttendee soft-deletes an attendee from all events in a series
func (q *Queries) DeleteSeriesAttendee(
	ctx context.Context,
	triggerEventUID uuid.UUID,
	accountID string,
) (int, int, error) {
	// Get current attendee
	current, err := q.GetAttendee(ctx, triggerEventUID, accountID)
	if err != nil {
		return 0, 0, err
	}

	if current.AttendeeGroupID == nil {
		return 0, 0, fmt.Errorf("attendee is not part of a series")
	}

	// If removing organizer, validate count on master
	if current.Role == RoleOrganizer {
		var masterUID uuid.UUID
		if current.MasterEventUID != nil {
			masterUID = *current.MasterEventUID
		} else {
			masterUID = triggerEventUID
		}

		count, err := q.CountOrganizers(ctx, masterUID)
		if err != nil {
			return 0, 0, err
		}
		if count <= 1 {
			return 0, 0, fmt.Errorf("cannot remove last organizer from event series")
		}
	}

	tx, err := q.pool.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil &&
			!errors.Is(rollbackErr, sql.ErrTxDone) {
			logger.Error("failed to rollback transaction: %v", rollbackErr)
		}
	}()

	// Soft delete attendees
	deleteQuery := `
		WITH trigger_info AS (
			SELECT ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE event_attendees ea
		SET archived = TRUE, archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE ea.event_uid = ce.event_uid
			AND ea.account_id = $2
			AND ea.attendee_group_id = $3
			AND ea.archived = FALSE
			AND (
				-- Archive from master
				ce.event_uid = ce.master_event_uid
				OR
				-- Archive from future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
		RETURNING ea.attendee_uid
	`

	rows, err := tx.QueryContext(
		ctx,
		deleteQuery,
		triggerEventUID,
		accountID,
		current.AttendeeGroupID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete series attendees: %w", err)
	}

	attendeeCount := 0
	for rows.Next() {
		attendeeCount++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				logger.Error("failed to close rows: %v", closeErr)
			}
			return 0, 0, fmt.Errorf("failed to scan uid: %w", err)
		}
	}
	if closeErr := rows.Close(); closeErr != nil {
		logger.Error("failed to close rows: %v", closeErr)
	}

	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	// Soft delete their reminders
	reminderQuery := `
		WITH trigger_info AS (
			SELECT ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE calendar_event_reminders r
		SET archived = TRUE, archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE r.event_uid = ce.event_uid
			AND r.account_id = $2
			AND r.archived = FALSE
			AND (
				-- Archive from master
				ce.event_uid = ce.master_event_uid
				OR
				-- Archive from future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
		RETURNING r.reminder_uid
	`

	reminderRows, err := tx.QueryContext(ctx, reminderQuery, triggerEventUID, accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to delete series reminders: %w", err)
	}

	reminderCount := 0
	for reminderRows.Next() {
		reminderCount++
		var uid uuid.UUID
		if err := reminderRows.Scan(&uid); err != nil {
			if closeErr := reminderRows.Close(); closeErr != nil {
				logger.Error("failed to close reminder rows: %v", closeErr)
			}
			return 0, 0, fmt.Errorf("failed to scan reminder uid: %w", err)
		}
	}
	if closeErr := reminderRows.Close(); closeErr != nil {
		logger.Error("failed to close reminder rows: %v", closeErr)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return attendeeCount, reminderCount, nil
}

// CopyAttendeesToNewEvent copies non-archived attendees from master to new instance
func (q *Queries) CopyAttendeesToNewEvent(
	ctx context.Context,
	masterEventUID, newEventUID uuid.UUID,
) (int, error) {
	query := `
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id,
			role, rsvp_status, metadata, archived
		)
		SELECT 
			$1, account_id, $2, attendee_group_id,
			role, rsvp_status, metadata, FALSE
		FROM event_attendees
		WHERE event_uid = $2 AND archived = FALSE
		RETURNING attendee_uid
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, newEventUID, masterEventUID)
	if err != nil {
		return 0, fmt.Errorf("failed to copy attendees: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		count++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			return 0, fmt.Errorf("failed to scan uid: %w", err)
		}
	}

	return count, rows.Err()
}

// TransferOwnershipSingle transfers ownership of a single event
func (q *Queries) TransferOwnershipSingle(
	ctx context.Context,
	eventUID uuid.UUID,
	currentOrganizerAccountID, newOrganizerAccountID string,
	newCalendarUID uuid.UUID,
) error {
	tx, err := q.pool.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil &&
			!errors.Is(rollbackErr, sql.ErrTxDone) {
			logger.Error("failed to rollback transaction: %v", rollbackErr)
		}
	}()

	// Add new organizer as attendee (or update if exists)
	addOrganizerQuery := `
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id,
			role, rsvp_status, metadata, archived
		)
		SELECT 
			$1, $2, ce.master_event_uid, NULL,
			'organizer', 'accepted', '{}', FALSE
		FROM calendar_events ce
		WHERE ce.event_uid = $1
		ON CONFLICT (event_uid, account_id)
		DO UPDATE SET
			role = 'organizer',
			rsvp_status = 'accepted',
			archived = FALSE,
			archived_ts = NULL,
			updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
	`
	_, err = tx.ExecContext(ctx, addOrganizerQuery, eventUID, newOrganizerAccountID)
	if err != nil {
		return fmt.Errorf("failed to add new organizer: %w", err)
	}

	// Demote current organizer to attendee
	demoteQuery := `
		UPDATE event_attendees
		SET role = 'attendee', updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $1 AND account_id = $2 AND role = 'organizer' AND archived = FALSE
	`
	_, err = tx.ExecContext(ctx, demoteQuery, eventUID, currentOrganizerAccountID)
	if err != nil {
		return fmt.Errorf("failed to demote current organizer: %w", err)
	}

	// Update event ownership
	updateEventQuery := `
		UPDATE calendar_events
		SET account_id = $1, calendar_uid = $2, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $3
	`
	_, err = tx.ExecContext(ctx, updateEventQuery, newOrganizerAccountID, newCalendarUID, eventUID)
	if err != nil {
		return fmt.Errorf("failed to update event ownership: %w", err)
	}

	return tx.Commit()
}

// TransferOwnershipSeries transfers ownership of all events in a series
func (q *Queries) TransferOwnershipSeries(
	ctx context.Context,
	triggerEventUID uuid.UUID,
	currentOrganizerAccountID, newOrganizerAccountID string,
	newCalendarUID uuid.UUID,
) (int, error) {
	tx, err := q.pool.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil &&
			!errors.Is(rollbackErr, sql.ErrTxDone) {
			logger.Error("failed to rollback transaction: %v", rollbackErr)
		}
	}()

	// Generate new group ID for new organizer attendees
	groupID := uuid.New()

	// Add new organizer to master + future instances
	addOrganizerQuery := `
		WITH trigger_info AS (
			SELECT 
				COALESCE(ce.master_event_uid, ce.event_uid) as effective_master_uid,
				ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		),
		affected_events AS (
			SELECT ce.event_uid, ce.master_event_uid
			FROM calendar_events ce, trigger_info ti
			WHERE 
				(ce.event_uid = ti.effective_master_uid)
				OR
				(ce.master_event_uid = ti.effective_master_uid 
				 AND ce.start_ts >= ti.trigger_start_ts
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
		)
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id,
			role, rsvp_status, metadata, archived
		)
		SELECT 
			ae.event_uid, $2, ae.master_event_uid, $3,
			'organizer', 'accepted', '{}', FALSE
		FROM affected_events ae
		ON CONFLICT (event_uid, account_id)
		DO UPDATE SET
			role = 'organizer',
			rsvp_status = 'accepted',
			attendee_group_id = EXCLUDED.attendee_group_id,
			archived = FALSE,
			archived_ts = NULL,
			updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		RETURNING attendee_uid
	`

	rows, err := tx.QueryContext(
		ctx,
		addOrganizerQuery,
		triggerEventUID,
		newOrganizerAccountID,
		groupID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add new organizer: %w", err)
	}

	count := 0
	for rows.Next() {
		count++
		var uid uuid.UUID
		if err := rows.Scan(&uid); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				logger.Error("failed to close rows: %v", closeErr)
			}
			return 0, fmt.Errorf("failed to scan uid: %w", err)
		}
	}
	if closeErr := rows.Close(); closeErr != nil {
		logger.Error("failed to close rows: %v", closeErr)
	}

	// Demote current organizer on master + future instances
	demoteQuery := `
		WITH trigger_info AS (
			SELECT ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE event_attendees ea
		SET role = 'attendee', updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE ea.event_uid = ce.event_uid
			AND ea.account_id = $2
			AND ea.role = 'organizer'
			AND ea.archived = FALSE
			AND (
				ce.event_uid = ce.master_event_uid
				OR
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
	`
	_, err = tx.ExecContext(ctx, demoteQuery, triggerEventUID, currentOrganizerAccountID)
	if err != nil {
		return 0, fmt.Errorf("failed to demote current organizer: %w", err)
	}

	// Update event ownership on master + future instances
	updateEventQuery := `
		WITH trigger_info AS (
			SELECT 
				COALESCE(ce.master_event_uid, ce.event_uid) as effective_master_uid,
				ce.start_ts as trigger_start_ts
			FROM calendar_events ce
			WHERE ce.event_uid = $1
		)
		UPDATE calendar_events ce
		SET account_id = $2, calendar_uid = $3, updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM trigger_info ti
		WHERE 
			(ce.event_uid = ti.effective_master_uid)
			OR
			(ce.master_event_uid = ti.effective_master_uid 
			 AND ce.start_ts >= ti.trigger_start_ts
			 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
	`
	_, err = tx.ExecContext(
		ctx,
		updateEventQuery,
		triggerEventUID,
		newOrganizerAccountID,
		newCalendarUID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to update event ownership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return count, nil
}

// CreateSingleAttendeeTx creates a single attendee within a transaction
func (q *Queries) CreateSingleAttendeeTx(
	ctx context.Context,
	tx *sql.Tx,
	eventUID uuid.UUID,
	accountID, role string,
	metadata json.RawMessage,
) error {
	// metadata is NOT NULL in DB, use empty object if nil or empty
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO event_attendees (
			event_uid, account_id, master_event_uid, attendee_group_id, 
			role, rsvp_status, metadata, archived
		)
		VALUES ($1, $2, NULL, NULL, $3, $4, $5, FALSE)
		ON CONFLICT (event_uid, account_id) 
		DO UPDATE SET 
			archived = FALSE,
			archived_ts = NULL,
			role = EXCLUDED.role,
			metadata = EXCLUDED.metadata,
			updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
	`

	rsvpStatus := RSVPPending
	if role == RoleOrganizer {
		rsvpStatus = RSVPAccepted
	}

	_, err := tx.ExecContext(ctx, query, eventUID, accountID, role, rsvpStatus, metadata)
	return err
}
