package calendar_member

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// CalendarMember represents a member of a calendar
type CalendarMember struct {
	AccountID   string    `json:"account_id"`
	CalendarUID uuid.UUID `json:"calendar_uid"`
	Status      string    `json:"status"` // pending, confirmed
	Role        string    `json:"role"`   // read, write
	InvitedBy   string    `json:"invited_by"`
	InvitedAtTs int64     `json:"invited_at_ts"`
	UpdatedTs   int64     `json:"updated_ts"`
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateCalendarMember adds a member to a calendar
func (q *Queries) CreateCalendarMember(ctx context.Context, member *CalendarMember) error {
	query := `
		INSERT INTO calendar_members (account_id, calendar_uid, status, role, invited_by, invited_at_ts, updated_ts)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := q.pool.DB().ExecContext(ctx, query,
		member.AccountID,
		member.CalendarUID,
		member.Status,
		member.Role,
		member.InvitedBy,
		member.InvitedAtTs,
		member.UpdatedTs,
	)

	return err
}

// CreateCalendarMembers adds multiple members to a calendar in a single transaction
func (q *Queries) CreateCalendarMembers(ctx context.Context, members []*CalendarMember) error {
	if len(members) == 0 {
		return nil
	}

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

	query := `
		INSERT INTO calendar_members (account_id, calendar_uid, status, role, invited_by, invited_at_ts, updated_ts)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			logger.Error("failed to close statement: %v", closeErr)
		}
	}()

	for _, member := range members {
		_, err := stmt.ExecContext(ctx,
			member.AccountID,
			member.CalendarUID,
			member.Status,
			member.Role,
			member.InvitedBy,
			member.InvitedAtTs,
			member.UpdatedTs,
		)
		if err != nil {
			return fmt.Errorf("failed to insert member %s: %w", member.AccountID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetCalendarMembers retrieves all members for a calendar
func (q *Queries) GetCalendarMembers(
	ctx context.Context,
	calendarUID uuid.UUID,
) ([]*CalendarMember, error) {
	query := `
		SELECT account_id, calendar_uid, status, role, invited_by, invited_at_ts, updated_ts
		FROM calendar_members
		WHERE calendar_uid = $1
		ORDER BY invited_at_ts DESC
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, calendarUID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	members := make([]*CalendarMember, 0)
	for rows.Next() {
		var member CalendarMember
		err := rows.Scan(
			&member.AccountID,
			&member.CalendarUID,
			&member.Status,
			&member.Role,
			&member.InvitedBy,
			&member.InvitedAtTs,
			&member.UpdatedTs,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, &member)
	}

	return members, nil
}

// GetMemberCalendars retrieves all calendars where the user is a confirmed member
func (q *Queries) GetMemberCalendars(
	ctx context.Context,
	accountID string,
	limit int,
	offset int,
) ([]uuid.UUID, error) {
	query := `
		SELECT calendar_uid
		FROM calendar_members
		WHERE account_id = $1 AND status = 'confirmed'
		ORDER BY invited_at_ts DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Error("Failed to close rows: %v", err)
		}
	}()

	calendarUIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var calendarUID uuid.UUID
		if err := rows.Scan(&calendarUID); err != nil {
			return nil, err
		}
		calendarUIDs = append(calendarUIDs, calendarUID)
	}

	return calendarUIDs, nil
}

// GetCalendarMember retrieves a specific member of a calendar
func (q *Queries) GetCalendarMember(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
) (*CalendarMember, error) {
	query := `
		SELECT account_id, calendar_uid, status, role, invited_by, invited_at_ts, updated_ts
		FROM calendar_members
		WHERE account_id = $1 AND calendar_uid = $2
	`

	var member CalendarMember
	err := q.pool.DB().QueryRowContext(ctx, query, accountID, calendarUID).Scan(
		&member.AccountID,
		&member.CalendarUID,
		&member.Status,
		&member.Role,
		&member.InvitedBy,
		&member.InvitedAtTs,
		&member.UpdatedTs,
	)

	if err != nil {
		return nil, err
	}

	return &member, nil
}

// UpdateMemberStatus updates the status of a calendar member
func (q *Queries) UpdateMemberStatus(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
	status string,
	updatedTs int64,
) error {
	query := `
		UPDATE calendar_members
		SET status = $3, updated_ts = $4
		WHERE account_id = $1 AND calendar_uid = $2
	`

	result, err := q.pool.DB().ExecContext(ctx, query, accountID, calendarUID, status, updatedTs)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateMemberRole updates the role of a calendar member
func (q *Queries) UpdateMemberRole(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
	role string,
	updatedTs int64,
) error {
	query := `
		UPDATE calendar_members
		SET role = $3, updated_ts = $4
		WHERE account_id = $1 AND calendar_uid = $2
	`

	result, err := q.pool.DB().ExecContext(ctx, query, accountID, calendarUID, role, updatedTs)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteCalendarMember removes a member from a calendar
func (q *Queries) DeleteCalendarMember(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
) error {
	query := `DELETE FROM calendar_members WHERE account_id = $1 AND calendar_uid = $2`
	result, err := q.pool.DB().ExecContext(ctx, query, accountID, calendarUID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// IsMemberOfCalendar checks if a user is the owner or a confirmed member of a calendar
func (q *Queries) IsMemberOfCalendar(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
) (bool, error) {
	// Check if user is the owner
	ownerQuery := `SELECT account_id FROM calendars WHERE calendar_uid = $1 AND account_id = $2`
	var ownerID string
	err := q.pool.DB().QueryRowContext(ctx, ownerQuery, calendarUID, accountID).Scan(&ownerID)
	if err == nil {
		return true, nil // User is the owner
	}
	if err != sql.ErrNoRows {
		return false, err
	}

	// Check if user is a confirmed member
	memberQuery := `SELECT account_id FROM calendar_members WHERE calendar_uid = $1 AND account_id = $2 AND status = 'confirmed'`
	var memberID string
	err = q.pool.DB().QueryRowContext(ctx, memberQuery, calendarUID, accountID).Scan(&memberID)
	if err == nil {
		return true, nil // User is a confirmed member
	}
	if err == sql.ErrNoRows {
		return false, nil // User is not a member
	}

	return false, err
}

// GetMemberRole gets the role of a member for a calendar
// Returns empty string if not a member or owner
func (q *Queries) GetMemberRole(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
) (string, error) {
	// Check if user is the owner (owners have write access)
	ownerQuery := `SELECT account_id FROM calendars WHERE calendar_uid = $1 AND account_id = $2`
	var ownerID string
	err := q.pool.DB().QueryRowContext(ctx, ownerQuery, calendarUID, accountID).Scan(&ownerID)
	if err == nil {
		return "write", nil // Owner has write access
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Check member role
	memberQuery := `SELECT role FROM calendar_members WHERE calendar_uid = $1 AND account_id = $2 AND status = 'confirmed'`
	var role string
	err = q.pool.DB().QueryRowContext(ctx, memberQuery, calendarUID, accountID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil // Not a member
	}
	if err != nil {
		return "", err
	}

	return role, nil
}

// IsCalendarOwner checks if a user is the owner of a calendar
func (q *Queries) IsCalendarOwner(
	ctx context.Context,
	accountID string,
	calendarUID uuid.UUID,
) (bool, error) {
	query := `SELECT account_id FROM calendars WHERE calendar_uid = $1 AND account_id = $2`
	var ownerID string
	err := q.pool.DB().QueryRowContext(ctx, query, calendarUID, accountID).Scan(&ownerID)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}
