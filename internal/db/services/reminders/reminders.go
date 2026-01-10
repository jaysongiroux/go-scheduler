package reminder

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

// TODO: replace q with Reminder instead - maybe?
// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateSingleReminder creates a one-off reminder for a specific event (scope="single")
func (q *Queries) CreateSingleReminder(
	ctx context.Context,
	rm *event.Reminder,
) (*event.Reminder, error) {
	query := `
		INSERT INTO calendar_event_reminders (
			reminder_uid, event_uid, account_id, master_event_uid,
			reminder_group_id, offset_seconds, metadata, created_ts, archived
		)
		SELECT 
			gen_random_uuid(),
			$1,
			$2,
			ce.master_event_uid,
			NULL,
			$3,
			$4,
			EXTRACT(EPOCH FROM NOW())::BIGINT,
			false
		FROM calendar_events ce
		WHERE ce.event_uid = $1
		RETURNING reminder_uid, event_uid, account_id, master_event_uid,
		          reminder_group_id, offset_seconds, metadata, is_delivered,
		          delivered_ts, created_ts, archived, archived_ts
	`

	var reminder event.Reminder
	var masterEventUID, reminderGroupID, deliveredTs, archivedTs sql.NullString

	err := q.pool.DB().QueryRowContext(ctx, query,
		rm.EventUID,
		rm.AccountID,
		rm.OffsetSeconds,
		rm.Metadata,
	).Scan(
		&reminder.ReminderUID,
		&reminder.EventUID,
		&reminder.AccountID,
		&masterEventUID,
		&reminderGroupID,
		&reminder.OffsetSeconds,
		&reminder.Metadata,
		&reminder.IsDelivered,
		&deliveredTs,
		&reminder.CreatedTs,
		&reminder.Archived,
		&archivedTs,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable UUIDs
	if masterEventUID.Valid {
		uid, _ := uuid.Parse(masterEventUID.String)
		reminder.MasterEventUID = &uid
	}
	if reminderGroupID.Valid {
		uid, _ := uuid.Parse(reminderGroupID.String)
		reminder.ReminderGroupID = &uid
	}
	if deliveredTs.Valid {
		ts, _ := strconv.ParseInt(deliveredTs.String, 10, 64)
		reminder.DeliveredTs = &ts
	}
	if archivedTs.Valid {
		ts, _ := strconv.ParseInt(archivedTs.String, 10, 64)
		reminder.ArchivedTs = &ts
	}

	return &reminder, nil
}

// CreateSeriesReminder creates reminders for current and future events in a series (scope="all")
func (q *Queries) CreateSeriesReminder(
	ctx context.Context,
	rm *event.Reminder,
) (uuid.UUID, int, error) {
	groupID := uuid.New()

	query := `
		WITH trigger_event AS (
			SELECT 
				event_uid,
				master_event_uid,
				start_ts,
				is_recurring_instance,
				-- Determine the effective master UID
				CASE 
					WHEN master_event_uid IS NULL THEN event_uid  -- This event IS the master
					ELSE master_event_uid  -- This is an instance, use its master
				END as effective_master_uid
			FROM calendar_events
			WHERE event_uid = $1
		)
		INSERT INTO calendar_event_reminders (
			reminder_uid, event_uid, account_id, master_event_uid,
			reminder_group_id, offset_seconds, metadata, created_ts, archived
		)
		SELECT 
			gen_random_uuid(),
			ce.event_uid,
			$2,
			ce.master_event_uid,
			$3,
			$4,
			$5,
			EXTRACT(EPOCH FROM NOW())::BIGINT,
			false
		FROM calendar_events ce
		CROSS JOIN trigger_event te
		WHERE 
			ce.is_cancelled = FALSE
			AND (
				-- The master event itself (critical for worker!)
				(ce.event_uid = te.effective_master_uid)
				OR
				-- Current and future instances
				(ce.master_event_uid = te.effective_master_uid 
				 AND ce.is_recurring_instance = true
				 AND ce.start_ts >= te.start_ts)
			)
	`

	result, err := q.pool.DB().ExecContext(ctx, query,
		rm.EventUID,
		rm.AccountID,
		groupID,
		rm.OffsetSeconds,
		rm.Metadata,
	)
	if err != nil {
		return uuid.Nil, 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil, 0, err
	}
	return groupID, int(rowsAffected), nil
}

// UpdateSingleReminder updates a one-off reminder (scope="single")
func (q *Queries) UpdateSingleReminder(
	ctx context.Context,
	rm *event.Reminder,
) error {
	query := `
		UPDATE calendar_event_reminders
		SET 
			offset_seconds = $2,
			metadata = $3,
			is_delivered = false,
			delivered_ts = NULL
		WHERE reminder_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query, rm.ReminderUID, rm.OffsetSeconds, rm.Metadata)
	return err
}

// UpdateSeriesReminder updates reminders for current and future events in a series (scope="all")
func (q *Queries) UpdateSeriesReminder(
	ctx context.Context,
	rm *event.Reminder,
) (int, error) {
	query := `
		WITH trigger_info AS (
			SELECT 
				r.reminder_group_id,
				ce.start_ts as trigger_start_ts
			FROM calendar_event_reminders r
			JOIN calendar_events ce ON r.event_uid = ce.event_uid
			WHERE r.reminder_uid = $1
		)
		UPDATE calendar_event_reminders r
		SET 
			offset_seconds = $2,
			metadata = $3,
			is_delivered = false,
			delivered_ts = NULL
		FROM calendar_events ce, trigger_info ti
		WHERE 
			r.event_uid = ce.event_uid
			AND r.reminder_group_id = ti.reminder_group_id
			AND r.reminder_group_id IS NOT NULL
			AND (
				-- Update master event (for worker to copy)
				ce.event_uid = ce.master_event_uid
				OR
				-- Update future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
	`

	result, err := q.pool.DB().ExecContext(ctx, query, rm.ReminderUID, rm.OffsetSeconds, rm.Metadata)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	return int(rowsAffected), err
}

// DeleteSingleReminder soft deletes a one-off reminder (scope="single")
func (q *Queries) DeleteSingleReminder(ctx context.Context, reminderUID uuid.UUID) error {
	query := `
		UPDATE calendar_event_reminders
		SET 
			archived = true,
			archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE reminder_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query, reminderUID)
	return err
}

// DeleteSeriesReminder soft deletes reminders for current and future events in a series (scope="all")
func (q *Queries) DeleteSeriesReminder(ctx context.Context, reminderUID uuid.UUID) (int, error) {
	query := `
		WITH trigger_info AS (
			SELECT 
				r.reminder_group_id,
				ce.start_ts as trigger_start_ts
			FROM calendar_event_reminders r
			JOIN calendar_events ce ON r.event_uid = ce.event_uid
			WHERE r.reminder_uid = $1
		)
		UPDATE calendar_event_reminders r
		SET 
			archived = true,
			archived_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM calendar_events ce, trigger_info ti
		WHERE 
			r.event_uid = ce.event_uid
			AND r.reminder_group_id = ti.reminder_group_id
			AND r.reminder_group_id IS NOT NULL
			AND (
				-- Archive from master (prevents future copies)
				ce.event_uid = ce.master_event_uid
				OR
				-- Archive from future instances only
				(ce.start_ts >= ti.trigger_start_ts 
				 AND ce.start_ts >= EXTRACT(EPOCH FROM NOW())::BIGINT)
			)
	`

	result, err := q.pool.DB().ExecContext(ctx, query, reminderUID)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	return int(rowsAffected), err
}

// GetEventReminders retrieves all non-archived reminders for an event
func (q *Queries) GetEventReminders(
	ctx context.Context,
	eventUID uuid.UUID,
	accountID *string,
) ([]*event.Reminder, error) {
	query := `
		SELECT 
			reminder_uid, event_uid, account_id, master_event_uid,
			reminder_group_id, offset_seconds, metadata, is_delivered,
			delivered_ts, created_ts, archived, archived_ts
		FROM calendar_event_reminders
		WHERE event_uid = $1
		  AND archived = false
		  AND ($2::TEXT IS NULL OR account_id = $2)
		ORDER BY offset_seconds DESC
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, eventUID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reminders := make([]*event.Reminder, 0)
	for rows.Next() {
		var r event.Reminder
		var masterEventUID, reminderGroupID, deliveredTs, archivedTs sql.NullString

		err := rows.Scan(
			&r.ReminderUID,
			&r.EventUID,
			&r.AccountID,
			&masterEventUID,
			&reminderGroupID,
			&r.OffsetSeconds,
			&r.Metadata,
			&r.IsDelivered,
			&deliveredTs,
			&r.CreatedTs,
			&r.Archived,
			&archivedTs,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if masterEventUID.Valid {
			uid, _ := uuid.Parse(masterEventUID.String)
			r.MasterEventUID = &uid
		}
		if reminderGroupID.Valid {
			uid, _ := uuid.Parse(reminderGroupID.String)
			r.ReminderGroupID = &uid
		}
		if deliveredTs.Valid {
			ts, _ := strconv.ParseInt(deliveredTs.String, 10, 64)
			r.DeliveredTs = &ts
		}
		if archivedTs.Valid {
			ts, _ := strconv.ParseInt(archivedTs.String, 10, 64)
			r.ArchivedTs = &ts
		}

		reminders = append(reminders, &r)
	}

	return reminders, rows.Err()
}

// CopyRemindersToNewEvent copies all non-archived reminders from master event to a new instance
// Used by the worker when generating new recurring event instances
func (q *Queries) CopyRemindersToNewEvent(
	ctx context.Context,
	masterEventUID uuid.UUID,
	newEventUID uuid.UUID,
) (int, error) {
	query := `
		INSERT INTO calendar_event_reminders (
			reminder_uid, event_uid, account_id, master_event_uid,
			reminder_group_id, offset_seconds, metadata, created_ts, archived
		)
		SELECT 
			gen_random_uuid(),
			$1,
			r.account_id,
			$2,
			r.reminder_group_id,
			r.offset_seconds,
			r.metadata,
			EXTRACT(EPOCH FROM NOW())::BIGINT,
			false
		FROM calendar_event_reminders r
		WHERE 
			r.event_uid = $2
			AND r.archived = false
	`

	result, err := q.pool.DB().ExecContext(ctx, query, newEventUID, masterEventUID)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	return int(rowsAffected), err
}

// GetReminderByUID retrieves a single reminder by its UID
func (q *Queries) GetReminderByUID(ctx context.Context, reminderUID uuid.UUID) (*event.Reminder, error) {
	query := `
		SELECT 
			reminder_uid, event_uid, account_id, master_event_uid,
			reminder_group_id, offset_seconds, metadata, is_delivered,
			delivered_ts, created_ts, archived, archived_ts
		FROM calendar_event_reminders
		WHERE reminder_uid = $1
	`

	var r event.Reminder
	var masterEventUID, reminderGroupID, deliveredTs, archivedTs sql.NullString

	err := q.pool.DB().QueryRowContext(ctx, query, reminderUID).Scan(
		&r.ReminderUID,
		&r.EventUID,
		&r.AccountID,
		&masterEventUID,
		&reminderGroupID,
		&r.OffsetSeconds,
		&r.Metadata,
		&r.IsDelivered,
		&deliveredTs,
		&r.CreatedTs,
		&r.Archived,
		&archivedTs,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if masterEventUID.Valid {
		uid, _ := uuid.Parse(masterEventUID.String)
		r.MasterEventUID = &uid
	}
	if reminderGroupID.Valid {
		uid, _ := uuid.Parse(reminderGroupID.String)
		r.ReminderGroupID = &uid
	}
	if deliveredTs.Valid {
		ts, _ := strconv.ParseInt(deliveredTs.String, 10, 64)
		r.DeliveredTs = &ts
	}
	if archivedTs.Valid {
		ts, _ := strconv.ParseInt(archivedTs.String, 10, 64)
		r.ArchivedTs = &ts
	}

	return &r, nil
}

// MarkReminderDelivered marks a reminder as delivered
func (q *Queries) MarkReminderDelivered(ctx context.Context, reminderUID uuid.UUID) error {
	query := `
		UPDATE calendar_event_reminders
		SET 
			is_delivered = true,
			delivered_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE reminder_uid = $1
	`

	_, err := q.pool.DB().ExecContext(ctx, query, reminderUID)
	return err
}

// GetDueRemindersWithEvents retrieves reminders that need to be delivered along with their event data
// Used by the reminder trigger worker to send webhooks
func (q *Queries) GetDueRemindersWithEvents(
	ctx context.Context,
	beforeTimestamp int64,
	limit int,
) ([]*event.ReminderWithEvent, error) {
	query := `
		SELECT 
			r.reminder_uid,
			r.offset_seconds,
			r.metadata as reminder_metadata,
			(ce.start_ts + r.offset_seconds) as remind_at_ts,
			ce.event_uid,
			ce.calendar_uid,
			ce.account_id,
			ce.start_ts,
			ce.metadata as event_metadata
		FROM calendar_event_reminders r
		JOIN calendar_events ce ON r.event_uid = ce.event_uid
		WHERE 
			r.archived = false
			AND r.is_delivered = false
			AND ce.is_cancelled = false
			AND (ce.start_ts + r.offset_seconds) <= $1
		ORDER BY (ce.start_ts + r.offset_seconds) ASC
		LIMIT $2
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, beforeTimestamp, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*event.ReminderWithEvent
	for rows.Next() {
		var rwe event.ReminderWithEvent
		var reminderMetadataJSON []byte
		var eventMetadataJSON []byte

		err := rows.Scan(
			&rwe.ReminderUID,
			&rwe.OffsetSeconds,
			&reminderMetadataJSON,
			&rwe.RemindAtTs,
			&rwe.EventUID,
			&rwe.CalendarUID,
			&rwe.AccountID,
			&rwe.StartTs,
			&eventMetadataJSON,
		)
		if err != nil {
			return nil, err
		}

		// Parse reminder metadata
		if len(reminderMetadataJSON) > 0 {
			var reminderMeta interface{}
			if err := json.Unmarshal(reminderMetadataJSON, &reminderMeta); err != nil {
				logger.Error("Failed to unmarshal reminder metadata: %v", err)
				rwe.ReminderMetadata = map[string]interface{}{}
			} else {
				rwe.ReminderMetadata = reminderMeta
			}
		} else {
			rwe.ReminderMetadata = map[string]interface{}{}
		}

		// Parse event metadata
		if len(eventMetadataJSON) > 0 {
			var eventMeta interface{}
			if err := json.Unmarshal(eventMetadataJSON, &eventMeta); err != nil {
				logger.Error("Failed to unmarshal event metadata: %v", err)
				rwe.EventMetadata = map[string]interface{}{}
			} else {
				rwe.EventMetadata = eventMeta
			}
		} else {
			rwe.EventMetadata = map[string]interface{}{}
		}

		results = append(results, &rwe)
	}

	return results, rows.Err()
}

// GetDueReminders retrieves reminders that need to be delivered
// Used by the worker to find reminders that should be sent
func (q *Queries) GetDueReminders(
	ctx context.Context,
	beforeTimestamp int64,
	limit int,
) ([]*event.Reminder, error) {
	query := `
		SELECT 
			r.reminder_uid, r.event_uid, r.account_id, r.master_event_uid,
			r.reminder_group_id, r.offset_seconds, r.metadata, r.is_delivered,
			r.delivered_ts, r.created_ts, r.archived, r.archived_ts
		FROM calendar_event_reminders r
		JOIN calendar_events ce ON r.event_uid = ce.event_uid
		WHERE 
			r.archived = false
			AND r.is_delivered = false
			AND ce.is_cancelled = false
			AND (ce.start_ts + r.offset_seconds) <= $1
		ORDER BY (ce.start_ts + r.offset_seconds) ASC
		LIMIT $2
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, beforeTimestamp, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []*event.Reminder
	for rows.Next() {
		var r event.Reminder
		var masterEventUID, reminderGroupID, deliveredTs, archivedTs sql.NullString

		err := rows.Scan(
			&r.ReminderUID,
			&r.EventUID,
			&r.AccountID,
			&masterEventUID,
			&reminderGroupID,
			&r.OffsetSeconds,
			&r.Metadata,
			&r.IsDelivered,
			&deliveredTs,
			&r.CreatedTs,
			&r.Archived,
			&archivedTs,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if masterEventUID.Valid {
			uid, _ := uuid.Parse(masterEventUID.String)
			r.MasterEventUID = &uid
		}
		if reminderGroupID.Valid {
			uid, _ := uuid.Parse(reminderGroupID.String)
			r.ReminderGroupID = &uid
		}
		if deliveredTs.Valid {
			ts, _ := strconv.ParseInt(deliveredTs.String, 10, 64)
			r.DeliveredTs = &ts
		}
		if archivedTs.Valid {
			ts, _ := strconv.ParseInt(archivedTs.String, 10, 64)
			r.ArchivedTs = &ts
		}

		reminders = append(reminders, &r)
	}

	return reminders, rows.Err()
}
