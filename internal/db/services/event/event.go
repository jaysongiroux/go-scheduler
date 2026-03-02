package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/rrule"
	"github.com/lib/pq"
	rr "github.com/teambition/rrule-go"
)

// RecurrenceStatus represents the status of a recurring event
type RecurrenceStatus string

const (
	RecurrenceStatusActive   RecurrenceStatus = "active"
	RecurrenceStatusInactive RecurrenceStatus = "inactive"
)

// Reminder represents a reminder for an event (now stored in separate table)
type Reminder struct {
	ReminderUID     uuid.UUID       `json:"reminder_uid"`
	EventUID        uuid.UUID       `json:"event_uid"`
	AccountID       string          `json:"account_id"`
	MasterEventUID  *uuid.UUID      `json:"master_event_uid,omitempty"`
	ReminderGroupID *uuid.UUID      `json:"reminder_group_id,omitempty"`
	OffsetSeconds   int64           `json:"offset_seconds"`
	Metadata        json.RawMessage `json:"metadata"`
	IsDelivered     bool            `json:"is_delivered"`
	DeliveredTs     *int64          `json:"delivered_ts,omitempty"`
	CreatedTs       int64           `json:"created_ts"`
	Archived        bool            `json:"archived"`
	ArchivedTs      *int64          `json:"archived_ts,omitempty"`
}

// CalculateRemindAtTs calculates when a reminder should fire
func (r *Reminder) CalculateRemindAtTs(eventStartTs int64) int64 {
	return eventStartTs + r.OffsetSeconds // offset_seconds is negative
}

// ReminderWithEvent combines reminder and event data for webhook delivery
type ReminderWithEvent struct {
	// Reminder fields
	ReminderUID      uuid.UUID   `json:"reminder_uid"`
	OffsetSeconds    int64       `json:"offset_seconds"`
	ReminderMetadata interface{} `json:"reminder_metadata"`
	RemindAtTs       int64       `json:"remind_at_ts"`

	// Event fields
	EventUID      uuid.UUID   `json:"event_uid"`
	CalendarUID   uuid.UUID   `json:"calendar_uid"`
	AccountID     string      `json:"account_id"`
	StartTs       int64       `json:"start_ts"`
	EventMetadata interface{} `json:"event_metadata"`
}

// Event represents the calendar_events table
type Event struct {
	EventUID    uuid.UUID `json:"event_uid"`
	CalendarUID uuid.UUID `json:"calendar_uid"`
	AccountID   string    `json:"account_id"`
	StartTs     int64     `json:"start_ts"`
	Duration    int64     `json:"duration"`
	EndTs       int64     `json:"end_ts"`
	CreatedTs   int64     `json:"created_ts"`
	UpdatedTs   int64     `json:"updated_ts"`

	// Timezone support for DST-aware scheduling
	// Timezone is an IANA timezone string (e.g., "America/New_York")
	// LocalStart is the wall-clock time in RFC3339 format without timezone offset
	Timezone   *string `json:"timezone,omitempty"`
	LocalStart *string `json:"local_start,omitempty"`

	// Recurrence tracking (master event fields)
	Recurrence       *rrule.Recurrence `json:"recurrence"`
	RecurrenceStatus *RecurrenceStatus `json:"recurrence_status"`
	RecurrenceEndTs  *int64            `json:"recurrence_end_ts"`
	ExDatesTs        []int64           `json:"exdates_ts"`

	// Instance tracking fields
	IsRecurringInstance bool       `json:"is_recurring_instance"`
	MasterEventUID      *uuid.UUID `json:"master_event_uid"`
	OriginalStartTs     *int64     `json:"original_start_ts"`

	// State tracking
	IsModified  bool `json:"is_modified"`
	IsCancelled bool `json:"is_cancelled"`

	// Metadata
	Metadata json.RawMessage `json:"metadata"`

	// Reminders - populated separately from calendar_event_reminders table
	// This field is NOT persisted in the events table
	Reminders []Reminder `json:"reminders,omitempty"`
}

// IsMasterEvent returns true if this is a master recurring event
func (e *Event) IsMasterEvent() bool {
	return e.Recurrence != nil && !e.IsRecurringInstance
}

// IsInstance returns true if this is an instance of a recurring event
func (e *Event) IsInstance() bool {
	return e.IsRecurringInstance && e.MasterEventUID != nil
}

// MarshalJSON customizes JSON serialization to set master_event_uid for master events
func (e *Event) MarshalJSON() ([]byte, error) {
	// Create an alias to avoid infinite recursion
	type Alias Event

	// If this is a master event, set master_event_uid to its own event_uid
	if e.IsMasterEvent() && e.MasterEventUID == nil {
		temp := struct {
			*Alias
			MasterEventUID *uuid.UUID `json:"master_event_uid"`
		}{
			Alias:          (*Alias)(e),
			MasterEventUID: &e.EventUID,
		}
		return json.Marshal(temp)
	}

	// Otherwise, use default serialization
	return json.Marshal((*Alias)(e))
}

// Queries holds the database connection pool
type Queries struct {
	pool *db.ConnectionPool
}

// New creates a new Queries instance
func New(pool *db.ConnectionPool) *Queries {
	return &Queries{pool: pool}
}

// CreateEvent creates a new event (non-recurring or master recurring event)
// Note: Reminders are NOT created here - use reminder repository separately
func (q *Queries) CreateEvent(ctx context.Context, event *Event) (Event, error) {
	// Validate recurrence rule if provided
	if err := rrule.ValidateRRule(event.Recurrence); err != nil {
		return Event{}, err
	}

	query := `
		-- CreateEvent ---
		INSERT INTO calendar_events (
			event_uid, calendar_uid, account_id, start_ts, duration, end_ts,
			created_ts, updated_ts, recurrence, recurrence_status, recurrence_end_ts,
			exdates_ts, is_recurring_instance, master_event_uid, original_start_ts,
			is_modified, is_cancelled, metadata, timezone, local_start
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING event_uid, calendar_uid, account_id, start_ts, duration, end_ts, 
		          created_ts, updated_ts, recurrence, recurrence_status, recurrence_end_ts, 
		          exdates_ts, is_recurring_instance, master_event_uid, original_start_ts, 
		          is_modified, is_cancelled, metadata, timezone, local_start
	`

	// Marshal recurrence to JSON (empty/nil becomes SQL NULL)
	var recurrenceJSON []byte
	if !rrule.IsRRuleEmpty(event.Recurrence) {
		var err error
		recurrenceJSON, err = json.Marshal(event.Recurrence)
		if err != nil {
			return Event{}, err
		}
	}

	// exdates_ts is NOT NULL in DB, use empty array if nil
	exdatesTs := event.ExDatesTs
	if exdatesTs == nil {
		exdatesTs = []int64{}
	}

	// metadata is NOT NULL in DB, use empty object if nil
	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	var createdEvent Event
	var recurrenceJSONReturned []byte
	var recurrenceStatusReturned sql.NullString
	var recurrenceEndTsReturned sql.NullInt64
	var masterEventUIDReturned *uuid.UUID
	var originalStartTsReturned sql.NullInt64
	var timezoneReturned sql.NullString
	var localStartReturned sql.NullString

	err := q.pool.DB().QueryRowContext(ctx, query,
		event.EventUID,
		event.CalendarUID,
		event.AccountID,
		event.StartTs,
		event.Duration,
		event.EndTs,
		event.CreatedTs,
		event.UpdatedTs,
		db.ToNullString(recurrenceJSON),
		db.ToNullableString(event.RecurrenceStatus),
		db.ToNullInt64(event.RecurrenceEndTs),
		pq.Array(exdatesTs),
		event.IsRecurringInstance,
		db.ToNullUUID(event.MasterEventUID),
		db.ToNullInt64(event.OriginalStartTs),
		event.IsModified,
		event.IsCancelled,
		metadata,
		db.ToNullableString(event.Timezone),
		db.ToNullableString(event.LocalStart),
	).Scan(
		&createdEvent.EventUID,
		&createdEvent.CalendarUID,
		&createdEvent.AccountID,
		&createdEvent.StartTs,
		&createdEvent.Duration,
		&createdEvent.EndTs,
		&createdEvent.CreatedTs,
		&createdEvent.UpdatedTs,
		&recurrenceJSONReturned,
		&recurrenceStatusReturned,
		&recurrenceEndTsReturned,
		pq.Array(&createdEvent.ExDatesTs),
		&createdEvent.IsRecurringInstance,
		&masterEventUIDReturned,
		&originalStartTsReturned,
		&createdEvent.IsModified,
		&createdEvent.IsCancelled,
		&createdEvent.Metadata,
		&timezoneReturned,
		&localStartReturned,
	)
	if err != nil {
		return Event{}, err
	}

	// Parse recurrence JSON
	if len(recurrenceJSONReturned) > 0 {
		if err := json.Unmarshal(recurrenceJSONReturned, &createdEvent.Recurrence); err != nil {
			return Event{}, err
		}
		if rrule.IsRRuleEmpty(createdEvent.Recurrence) {
			createdEvent.Recurrence = nil
		}
	}

	// Map nullable fields
	if recurrenceStatusReturned.Valid {
		status := RecurrenceStatus(recurrenceStatusReturned.String)
		createdEvent.RecurrenceStatus = &status
	}
	if recurrenceEndTsReturned.Valid {
		createdEvent.RecurrenceEndTs = &recurrenceEndTsReturned.Int64
	}
	if masterEventUIDReturned != nil {
		createdEvent.MasterEventUID = masterEventUIDReturned
	}
	if originalStartTsReturned.Valid {
		createdEvent.OriginalStartTs = &originalStartTsReturned.Int64
	}
	if timezoneReturned.Valid {
		createdEvent.Timezone = &timezoneReturned.String
	}
	if localStartReturned.Valid {
		createdEvent.LocalStart = &localStartReturned.String
	}

	// Reminders field is left empty - populate separately if needed
	createdEvent.Reminders = []Reminder{}

	return createdEvent, nil
}

// GetEvent retrieves an event by ID.
// For recurring instances the master's recurrence fields are returned via a
// LEFT JOIN so no extra round-trip is needed.
// Note: Reminders are NOT included - use reminder repository to fetch them separately.
func (q *Queries) GetEvent(ctx context.Context, eventUID uuid.UUID, _ *bool) (*Event, error) {
	query := `
		SELECT 
			e.event_uid, e.calendar_uid, e.account_id, e.start_ts, e.duration, e.end_ts,
			e.created_ts, e.updated_ts,
			COALESCE(e.recurrence, m.recurrence),
			COALESCE(e.recurrence_status, m.recurrence_status),
			COALESCE(e.recurrence_end_ts, m.recurrence_end_ts),
			e.exdates_ts, e.is_recurring_instance, e.master_event_uid, e.original_start_ts,
			e.is_modified, e.is_cancelled, e.metadata, e.timezone, e.local_start
		FROM calendar_events e
		LEFT JOIN calendar_events m
			ON e.is_recurring_instance = TRUE AND e.master_event_uid = m.event_uid
		WHERE e.event_uid = $1
	`

	var evt Event
	var recurrenceJSON []byte
	var recurrenceStatus sql.NullString
	var recurrenceEndTs sql.NullInt64
	var masterEventUID *uuid.UUID
	var originalStartTs sql.NullInt64
	var timezone sql.NullString
	var localStart sql.NullString

	err := q.pool.DB().QueryRowContext(ctx, query, eventUID).Scan(
		&evt.EventUID,
		&evt.CalendarUID,
		&evt.AccountID,
		&evt.StartTs,
		&evt.Duration,
		&evt.EndTs,
		&evt.CreatedTs,
		&evt.UpdatedTs,
		&recurrenceJSON,
		&recurrenceStatus,
		&recurrenceEndTs,
		pq.Array(&evt.ExDatesTs),
		&evt.IsRecurringInstance,
		&masterEventUID,
		&originalStartTs,
		&evt.IsModified,
		&evt.IsCancelled,
		&evt.Metadata,
		&timezone,
		&localStart,
	)
	if err != nil {
		return nil, err
	}

	// Parse recurrence JSON
	if len(recurrenceJSON) > 0 {
		if err := json.Unmarshal(recurrenceJSON, &evt.Recurrence); err != nil {
			return nil, err
		}
		if rrule.IsRRuleEmpty(evt.Recurrence) {
			evt.Recurrence = nil
		}
	}

	// Map nullable fields
	if recurrenceStatus.Valid {
		status := RecurrenceStatus(recurrenceStatus.String)
		evt.RecurrenceStatus = &status
	}
	if recurrenceEndTs.Valid {
		evt.RecurrenceEndTs = &recurrenceEndTs.Int64
	}
	if masterEventUID != nil {
		evt.MasterEventUID = masterEventUID
	}
	if originalStartTs.Valid {
		evt.OriginalStartTs = &originalStartTs.Int64
	}
	if timezone.Valid {
		evt.Timezone = &timezone.String
	}
	if localStart.Valid {
		evt.LocalStart = &localStart.String
	}

	// Reminders field is left empty - populate separately if needed
	evt.Reminders = []Reminder{}

	return &evt, nil
}

// GetCalendarEvents retrieves events for multiple calendars within a time range.
// Returns both non-recurring events and pre-generated instances within the range.
// For queries beyond the generation window, falls back to on-demand expansion.
//
// Note: Reminders are NOT included for performance reasons.
// Use GetEventReminders() to fetch reminders for specific events.
func (q *Queries) GetCalendarEvents(
	ctx context.Context,
	calendarUIDs []uuid.UUID,
	startTs, endTs int64,
) ([]*Event, error) {
	if len(calendarUIDs) == 0 {
		return []*Event{}, nil
	}

	var query string
	var args []interface{}

	// Build the IN clause for calendar UIDs
	placeholders := make([]string, len(calendarUIDs))
	args = make([]interface{}, 0, len(calendarUIDs)+2)

	for i, calendarUID := range calendarUIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, calendarUID)
	}

	startTsPlaceholder := fmt.Sprintf("$%d", len(calendarUIDs)+1)
	endTsPlaceholder := fmt.Sprintf("$%d", len(calendarUIDs)+2)
	args = append(args, startTs, endTs)

	// Fetch non-recurring events, pre-generated instances (with master recurrence
	// via LEFT JOIN), and master events for on-demand expansion — all in one query.
	query = fmt.Sprintf(`
		SELECT 
			e.event_uid, e.calendar_uid, e.account_id, e.start_ts, e.duration, e.end_ts,
			e.created_ts, e.updated_ts,
			COALESCE(e.recurrence, m.recurrence),
			COALESCE(e.recurrence_status, m.recurrence_status),
			COALESCE(e.recurrence_end_ts, m.recurrence_end_ts),
			e.exdates_ts, e.is_recurring_instance, e.master_event_uid, e.original_start_ts,
			e.is_modified, e.is_cancelled, e.metadata, e.timezone, e.local_start
		FROM calendar_events e
		LEFT JOIN calendar_events m
			ON e.is_recurring_instance = TRUE AND e.master_event_uid = m.event_uid
		WHERE e.calendar_uid IN (%s)
		AND e.is_cancelled = FALSE
		AND (
			-- Non-recurring events in time range
			(e.recurrence IS NULL AND e.is_recurring_instance = FALSE AND e.end_ts >= %s AND e.start_ts <= %s)
			OR 
			-- Pre-generated instances in time range
			(e.is_recurring_instance = TRUE AND e.end_ts >= %s AND e.start_ts <= %s)
			OR
			-- Master events (for potential on-demand expansion beyond generation window)
			(e.recurrence IS NOT NULL AND e.is_recurring_instance = FALSE)
		)
		ORDER BY e.start_ts ASC
	`, strings.Join(placeholders, ","), startTsPlaceholder, endTsPlaceholder, startTsPlaceholder, endTsPlaceholder)

	rows, err := q.pool.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	allEvents := make([]*Event, 0)
	masterEvents := make(map[uuid.UUID]*Event)   // Track master events for deduplication
	instancedMasters := make(map[uuid.UUID]bool) // Track which masters have instances in range

	for rows.Next() {
		evt, err := q.scanEventFromRows(rows)
		if err != nil {
			return nil, err
		}

		if evt.IsRecurringInstance {
			// This is a pre-generated instance
			allEvents = append(allEvents, evt)
			if evt.MasterEventUID != nil {
				instancedMasters[*evt.MasterEventUID] = true
			}
		} else if evt.IsMasterEvent() {
			// Store master events for potential on-demand expansion
			masterEvents[evt.EventUID] = evt
		} else {
			// Regular non-recurring event
			allEvents = append(allEvents, evt)
		}
	}

	if startTs > 0 && endTs > 0 {
		// Time range provided - expand on-demand if no instances exist.
		// Never include the master as a separate event in range-based list; return only instances
		// so each occurrence appears once (avoids duplicate on the first day).
		for masterUID, master := range masterEvents {
			if !instancedMasters[masterUID] {
				// No pre-generated instances for this master in the range
				// Fall back to on-demand expansion (instances only, not master)
				instances := q.expandRecurringEvent(master, startTs, endTs)
				if len(instances) > 0 {
					allEvents = append(allEvents, instances...)
				}
			}
			// When pre-generated instances exist they are already in allEvents; do not add master
		}
	} else {
		// No time range - return master events as-is (without expansion)
		for _, master := range masterEvents {
			allEvents = append(allEvents, master)
		}
	}

	// Remove events outside of the time range
	filteredEvents := make([]*Event, 0)
	for _, evt := range allEvents {
		if evt.StartTs >= startTs && evt.EndTs <= endTs {
			filteredEvents = append(filteredEvents, evt)
		}
	}

	return filteredEvents, nil
}

// scanEventFromRows scans an event from sql.Rows
func (q *Queries) scanEventFromRows(rows *sql.Rows) (*Event, error) {
	var evt Event
	var recurrenceJSON []byte
	var recurrenceStatus sql.NullString
	var recurrenceEndTs sql.NullInt64
	var masterEventUID *uuid.UUID
	var originalStartTs sql.NullInt64
	var timezone sql.NullString
	var localStart sql.NullString

	err := rows.Scan(
		&evt.EventUID,
		&evt.CalendarUID,
		&evt.AccountID,
		&evt.StartTs,
		&evt.Duration,
		&evt.EndTs,
		&evt.CreatedTs,
		&evt.UpdatedTs,
		&recurrenceJSON,
		&recurrenceStatus,
		&recurrenceEndTs,
		pq.Array(&evt.ExDatesTs),
		&evt.IsRecurringInstance,
		&masterEventUID,
		&originalStartTs,
		&evt.IsModified,
		&evt.IsCancelled,
		&evt.Metadata,
		&timezone,
		&localStart,
	)
	if err != nil {
		return nil, err
	}

	// Parse recurrence JSON
	if len(recurrenceJSON) > 0 {
		if err := json.Unmarshal(recurrenceJSON, &evt.Recurrence); err != nil {
			return nil, err
		}
		if rrule.IsRRuleEmpty(evt.Recurrence) {
			evt.Recurrence = nil
		}
	}

	// Map nullable fields
	if recurrenceStatus.Valid {
		status := RecurrenceStatus(recurrenceStatus.String)
		evt.RecurrenceStatus = &status
	}
	if recurrenceEndTs.Valid {
		evt.RecurrenceEndTs = &recurrenceEndTs.Int64
	}
	if masterEventUID != nil {
		evt.MasterEventUID = masterEventUID
	}
	if originalStartTs.Valid {
		evt.OriginalStartTs = &originalStartTs.Int64
	}
	if timezone.Valid {
		evt.Timezone = &timezone.String
	}
	if localStart.Valid {
		evt.LocalStart = &localStart.String
	}

	// Reminders field is left empty - populate separately if needed
	evt.Reminders = []Reminder{}

	return &evt, nil
}

// expandRecurringEvent expands a master event into virtual instances for on-demand queries.
// This is used as a fallback when pre-generated instances are not available.
// Uses timezone-aware expansion to correctly handle DST transitions.
func (q *Queries) expandRecurringEvent(
	event *Event,
	viewStart, viewEnd int64,
) []*Event {
	if event == nil || event.Recurrence == nil || event.Recurrence.Rule == "" {
		return nil
	}

	// Parse base RRULE into options
	opt, err := rr.StrToROption(event.Recurrence.Rule)
	if err != nil {
		logger.Error("Failed to parse recurrence rule: %v", err)
		return nil
	}

	// Load timezone for DST-aware expansion (default to UTC if not set)
	loc := time.UTC
	if event.Timezone != nil && *event.Timezone != "" {
		loc, err = time.LoadLocation(*event.Timezone)
		if err != nil {
			logger.Warn("Failed to load timezone %s, falling back to UTC: %v", *event.Timezone, err)
			loc = time.UTC
		}
	}

	// Apply DTSTART in local timezone for DST-aware recurrence expansion
	// If local_start is set, parse it in the timezone; otherwise use start_ts
	if event.LocalStart != nil && *event.LocalStart != "" {
		localTime, err := time.ParseInLocation("2006-01-02T15:04:05", *event.LocalStart, loc)
		if err == nil {
			opt.Dtstart = localTime
		} else {
			opt.Dtstart = time.Unix(event.StartTs, 0).In(loc)
		}
	} else {
		opt.Dtstart = time.Unix(event.StartTs, 0).In(loc)
	}

	// Build RRULE
	r, err := rr.NewRRule(*opt)
	if err != nil {
		logger.Error("Failed to build recurrence rule: %v", err)
		return nil
	}

	// Expand occurrences within the view window (in local timezone)
	occurrences := r.Between(
		time.Unix(viewStart, 0).In(loc),
		time.Unix(viewEnd, 0).In(loc),
		true,
	)

	// Build EXDATE lookup
	exDateMap := make(map[int64]bool, len(event.ExDatesTs))
	for _, ex := range event.ExDatesTs {
		exDateMap[ex] = true
	}

	instances := make([]*Event, 0, len(occurrences))
	for _, occ := range occurrences {
		// Convert occurrence to UTC for storage
		occTs := occ.UTC().Unix()

		// Skip excluded instances
		if exDateMap[occTs] {
			continue
		}

		// Compute local_start for this occurrence (format in event TZ for DST consistency)
		var localStart *string
		if event.Timezone != nil && *event.Timezone != "" {
			ls := occ.In(loc).Format("2006-01-02T15:04:05")
			localStart = &ls
		}

		// Create virtual instance (not persisted, for on-demand expansion only)
		instance := &Event{
			EventUID:            event.EventUID, // Same as master for virtual instances
			CalendarUID:         event.CalendarUID,
			AccountID:           event.AccountID,
			StartTs:             occTs,
			Duration:            event.Duration,
			EndTs:               occTs + event.Duration,
			CreatedTs:           event.CreatedTs,
			UpdatedTs:           event.UpdatedTs,
			Timezone:            event.Timezone,
			LocalStart:          localStart,
			Recurrence:          event.Recurrence,
			RecurrenceStatus:    event.RecurrenceStatus,
			RecurrenceEndTs:     event.RecurrenceEndTs,
			IsRecurringInstance: true,
			MasterEventUID:      &event.EventUID,
			OriginalStartTs:     &occTs,
			Metadata:            event.Metadata,
			Reminders:           []Reminder{},
		}

		instances = append(instances, instance)
	}

	return instances
}

// UpdateEvent updates an event (works for both master and instance events)
// Note: This does NOT update reminders - use reminder repository for that
func (q *Queries) UpdateEvent(ctx context.Context, event *Event) error {
	// Validate recurrence rule if provided
	if err := rrule.ValidateRRule(event.Recurrence); err != nil {
		return err
	}

	query := `
		UPDATE calendar_events
		SET calendar_uid = $2,
		    start_ts = $3,
		    duration = $4,
		    end_ts = $5,
		    recurrence = $6,
		    recurrence_status = $7,
		    recurrence_end_ts = $8,
		    exdates_ts = $9,
		    is_modified = $10,
		    is_cancelled = $11,
		    metadata = $12,
		    timezone = $13,
		    local_start = $14,
		    updated_ts = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE event_uid = $1
	`

	// Marshal recurrence to JSON (empty/nil becomes SQL NULL)
	var recurrenceJSON []byte
	if !rrule.IsRRuleEmpty(event.Recurrence) {
		var err error
		recurrenceJSON, err = json.Marshal(event.Recurrence)
		if err != nil {
			return err
		}
	}

	exdatesTs := event.ExDatesTs
	if exdatesTs == nil {
		exdatesTs = []int64{}
	}

	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	_, err := q.pool.DB().ExecContext(ctx, query,
		event.EventUID,
		event.CalendarUID,
		event.StartTs,
		event.Duration,
		event.EndTs,
		db.ToNullString(recurrenceJSON),
		db.ToNullableString(event.RecurrenceStatus),
		db.ToNullInt64(event.RecurrenceEndTs),
		pq.Array(exdatesTs),
		event.IsModified,
		event.IsCancelled,
		metadata,
		db.ToNullableString(event.Timezone),
		db.ToNullableString(event.LocalStart),
	)

	return err
}

// DeleteEvent deletes an event (cascades to instances if master)
func (q *Queries) DeleteEvent(ctx context.Context, eventUID uuid.UUID) error {
	query := `DELETE FROM calendar_events WHERE event_uid = $1`
	_, err := q.pool.DB().ExecContext(ctx, query, eventUID)
	return err
}

// GenerationWindow represents the time window for instance generation
type GenerationWindow struct {
	WindowDuration time.Duration
	BufferDuration time.Duration
}

// NewGenerationWindow creates a generation window with the given durations
func NewGenerationWindow(windowDuration, bufferDuration time.Duration) GenerationWindow {
	return GenerationWindow{
		WindowDuration: windowDuration,
		BufferDuration: bufferDuration,
	}
}

// CreateEventWithInstances creates a master recurring event and generates instances
// for the specified window. Returns the master event and generated instances.
// Note: Reminders must be created separately using the reminder repository
func (q *Queries) CreateEventWithInstances(
	ctx context.Context,
	event *Event,
	window GenerationWindow,
) (*Event, []*Event, error) {
	// Validate this is a recurring event
	if event.Recurrence == nil || event.Recurrence.Rule == "" {
		return nil, nil, nil
	}

	if err := rrule.ValidateRRule(event.Recurrence); err != nil {
		return nil, nil, err
	}

	// Set master event properties
	event.IsRecurringInstance = false
	event.MasterEventUID = nil
	event.OriginalStartTs = nil

	// Calculate recurrence status
	status := q.calculateRecurrenceStatus(event, window)
	event.RecurrenceStatus = &status

	// Create the master event
	createdEvent, err := q.CreateEvent(ctx, event)
	if err != nil {
		return nil, nil, err
	}
	*event = createdEvent

	// Generate instances for the window
	now := time.Now().UTC()
	windowEnd := now.Add(window.WindowDuration + window.BufferDuration)

	instances := q.generateInstances(event, event.StartTs, windowEnd.Unix())

	// Bulk insert the instances
	if len(instances) > 0 {
		if err := q.BulkInsertInstances(ctx, instances); err != nil {
			return nil, nil, err
		}
	}

	return event, instances, nil
}

// calculateRecurrenceStatus determines if a recurring event is active or inactive
// Uses timezone-aware handling for DST correctness.
func (q *Queries) calculateRecurrenceStatus(
	event *Event,
	window GenerationWindow,
) RecurrenceStatus {
	if event.Recurrence == nil || event.Recurrence.Rule == "" {
		return RecurrenceStatusInactive
	}

	// Parse RRULE to check for UNTIL or COUNT
	opt, err := rr.StrToROption(event.Recurrence.Rule)
	if err != nil {
		return RecurrenceStatusInactive
	}

	// If there's an UNTIL date
	if !opt.Until.IsZero() {
		windowEnd := time.Now().Add(window.WindowDuration)
		if opt.Until.Before(windowEnd) {
			return RecurrenceStatusInactive
		}
	}

	// If there's a COUNT limit, check if we'd exceed it within the window
	if opt.Count > 0 {
		// Load timezone for DST-aware expansion
		loc := time.UTC
		if event.Timezone != nil && *event.Timezone != "" {
			loc, err = time.LoadLocation(*event.Timezone)
			if err != nil {
				loc = time.UTC
			}
		}

		// Apply DTSTART in local timezone
		if event.LocalStart != nil && *event.LocalStart != "" {
			localTime, err := time.ParseInLocation("2006-01-02T15:04:05", *event.LocalStart, loc)
			if err == nil {
				opt.Dtstart = localTime
			} else {
				opt.Dtstart = time.Unix(event.StartTs, 0).In(loc)
			}
		} else {
			opt.Dtstart = time.Unix(event.StartTs, 0).In(loc)
		}

		r, err := rr.NewRRule(*opt)
		if err != nil {
			return RecurrenceStatusInactive
		}
		windowEnd := time.Now().Add(window.WindowDuration)
		occurrences := r.Between(opt.Dtstart, windowEnd.In(loc), true)
		if len(occurrences) >= opt.Count {
			return RecurrenceStatusInactive
		}
	}

	return RecurrenceStatusActive
}

// generateInstances creates instance events from a master event within a time range
// Note: Reminders are NOT copied here - use reminder repository's CopyRemindersToNewEvent
// Uses timezone-aware expansion to correctly handle DST transitions.
func (q *Queries) generateInstances(master *Event, startTs, endTs int64) []*Event {
	if master == nil || master.Recurrence == nil || master.Recurrence.Rule == "" {
		return nil
	}

	opt, err := rr.StrToROption(master.Recurrence.Rule)
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

	r, err := rr.NewRRule(*opt)
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
	instances := make([]*Event, 0, len(occurrences))

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

		instance := &Event{
			EventUID:            uuid.New(),
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
			Reminders:           []Reminder{}, // Reminders copied separately by reminder repository
		}

		instances = append(instances, instance)
	}

	return instances
}

// BulkInsertInstances inserts multiple event instances in a single transaction.
// Uses ON CONFLICT DO NOTHING to handle race conditions safely.
// Note: Does NOT copy reminders - use reminder repository's CopyRemindersToNewEvent
func (q *Queries) BulkInsertInstances(ctx context.Context, instances []*Event) error {
	if len(instances) == 0 {
		return nil
	}

	// Process in batches of 1000
	batchSize := 1000
	for i := 0; i < len(instances); i += batchSize {
		end := i + batchSize
		if end > len(instances) {
			end = len(instances)
		}
		batch := instances[i:end]

		if err := q.insertInstanceBatch(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func (q *Queries) insertInstanceBatch(ctx context.Context, instances []*Event) error {
	if len(instances) == 0 {
		return nil
	}

	query := `
		INSERT INTO calendar_events (
			event_uid, calendar_uid, account_id, start_ts, duration, end_ts,
			created_ts, updated_ts, is_recurring_instance, master_event_uid,
			original_start_ts, is_modified, is_cancelled, exdates_ts, metadata,
			timezone, local_start
		) VALUES `

	values := make([]interface{}, 0, len(instances)*17)
	placeholders := make([]string, 0, len(instances))

	for i, inst := range instances {
		base := i * 17
		placeholders = append(placeholders,
			"($"+itoa(base+1)+", $"+itoa(base+2)+", $"+itoa(base+3)+", $"+itoa(base+4)+
				", $"+itoa(base+5)+", $"+itoa(base+6)+", $"+itoa(base+7)+", $"+itoa(base+8)+
				", $"+itoa(base+9)+", $"+itoa(base+10)+", $"+itoa(base+11)+", $"+itoa(base+12)+
				", $"+itoa(base+13)+", $"+itoa(base+14)+", $"+itoa(base+15)+", $"+itoa(base+16)+
				", $"+itoa(base+17)+")")

		metadata := inst.Metadata
		if len(metadata) == 0 {
			metadata = json.RawMessage("{}")
		}

		values = append(values,
			inst.EventUID,
			inst.CalendarUID,
			inst.AccountID,
			inst.StartTs,
			inst.Duration,
			inst.EndTs,
			inst.CreatedTs,
			inst.UpdatedTs,
			inst.IsRecurringInstance,
			db.ToNullUUID(inst.MasterEventUID),
			db.ToNullInt64(inst.OriginalStartTs),
			inst.IsModified,
			inst.IsCancelled,
			pq.Array([]int64{}),
			metadata,
			db.ToNullableString(inst.Timezone),
			db.ToNullableString(inst.LocalStart),
		)
	}

	query += strings.Join(placeholders, ", ")
	query += " ON CONFLICT (master_event_uid, original_start_ts) WHERE is_recurring_instance = TRUE DO NOTHING"

	_, err := q.pool.DB().ExecContext(ctx, query, values...)
	return err
}

// DeleteFutureInstances deletes all instances of a master event after the given timestamp
func (q *Queries) DeleteFutureInstances(
	ctx context.Context,
	masterUID uuid.UUID,
	afterTs int64,
) error {
	query := `
		DELETE FROM calendar_events 
		WHERE master_event_uid = $1 
		AND is_recurring_instance = TRUE 
		AND start_ts >= $2
	`
	_, err := q.pool.DB().ExecContext(ctx, query, masterUID, afterTs)
	return err
}

// GetFutureInstances retrieves all instances of a master event from the given timestamp onwards
func (q *Queries) GetFutureInstances(
	ctx context.Context,
	masterUID uuid.UUID,
	fromTs int64,
) ([]*Event, error) {
	query := `
		SELECT 
			event_uid, calendar_uid, account_id, start_ts, duration, end_ts,
			created_ts, updated_ts, recurrence, recurrence_status, recurrence_end_ts,
			exdates_ts, is_recurring_instance, master_event_uid, original_start_ts,
			is_modified, is_cancelled, metadata, timezone, local_start
		FROM calendar_events
		WHERE master_event_uid = $1
		AND is_recurring_instance = TRUE
		AND start_ts >= $2
		AND is_cancelled = FALSE
		ORDER BY start_ts ASC
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, masterUID, fromTs)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	instances := make([]*Event, 0)
	for rows.Next() {
		evt, err := q.scanEventFromRows(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, evt)
	}

	return instances, nil
}

// GetMasterEvent retrieves the master event for a given instance
func (q *Queries) GetMasterEvent(ctx context.Context, instanceUID uuid.UUID) (*Event, error) {
	// First get the instance to find the master_event_uid
	instance, err := q.GetEvent(ctx, instanceUID, nil)
	if err != nil {
		return nil, err
	}

	if !instance.IsRecurringInstance || instance.MasterEventUID == nil {
		return nil, sql.ErrNoRows
	}

	return q.GetEvent(ctx, *instance.MasterEventUID, nil)
}

// AddExDate adds an exclusion date to a master event (for single instance deletion)
func (q *Queries) AddExDate(ctx context.Context, masterUID uuid.UUID, exdateTs int64) error {
	query := `
		UPDATE calendar_events 
		SET exdates_ts = array_append(exdates_ts, $2)
		WHERE event_uid = $1 
		AND is_recurring_instance = FALSE
	`
	_, err := q.pool.DB().ExecContext(ctx, query, masterUID, exdateTs)
	return err
}

// GetActiveRecurringEvents retrieves master events that need instance generation.
// Used by the recurring generation worker.
func (q *Queries) GetActiveRecurringEvents(ctx context.Context, limit int) ([]*Event, error) {
	query := `
		SELECT e.event_uid, e.calendar_uid, e.account_id, e.start_ts, e.duration, e.end_ts,
			e.created_ts, e.updated_ts, e.recurrence, e.recurrence_status, e.recurrence_end_ts,
			e.exdates_ts, e.is_recurring_instance, e.master_event_uid, e.original_start_ts,
			e.is_modified, e.is_cancelled, e.metadata, e.timezone, e.local_start
		FROM calendar_events e
		LEFT JOIN (
			SELECT master_event_uid, MAX(start_ts) as latest_instance
			FROM calendar_events
			WHERE is_recurring_instance = TRUE
			GROUP BY master_event_uid
		) i ON e.event_uid = i.master_event_uid
		WHERE e.recurrence_status = 'active'
		AND e.is_recurring_instance = FALSE
		AND e.is_cancelled = FALSE
		AND (
			i.latest_instance IS NULL 
			OR i.latest_instance < (EXTRACT(EPOCH FROM NOW()) + 31536000)
		)
		LIMIT $1
	`

	rows, err := q.pool.DB().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("failed to close rows: %v", closeErr)
		}
	}()

	events := make([]*Event, 0)
	for rows.Next() {
		evt, err := q.scanEventFromRows(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, evt)
	}

	return events, nil
}

// GetLatestInstanceTimestamp gets the latest instance start_ts for a master event
func (q *Queries) GetLatestInstanceTimestamp(
	ctx context.Context,
	masterUID uuid.UUID,
) (int64, error) {
	query := `
		SELECT COALESCE(MAX(start_ts), 0)
		FROM calendar_events
		WHERE master_event_uid = $1
		AND is_recurring_instance = TRUE
	`
	var latestTs int64
	err := q.pool.DB().QueryRowContext(ctx, query, masterUID).Scan(&latestTs)
	return latestTs, err
}

// UpdateRecurrenceStatus updates the recurrence_status of a master event
func (q *Queries) UpdateRecurrenceStatus(
	ctx context.Context,
	masterUID uuid.UUID,
	status RecurrenceStatus,
) error {
	query := `
		UPDATE calendar_events 
		SET recurrence_status = $2
		WHERE event_uid = $1 
		AND is_recurring_instance = FALSE
	`
	_, err := q.pool.DB().ExecContext(ctx, query, masterUID, string(status))
	return err
}

func (q *Queries) ToggleCancelledStatusEvent(ctx context.Context, eventUID uuid.UUID) error {
	query := `
		UPDATE calendar_events 
		SET is_cancelled = NOT is_cancelled
		WHERE event_uid = $1
	`
	_, err := q.pool.DB().ExecContext(ctx, query, eventUID)
	return err
}

// CancelInstance marks an instance as cancelled (soft delete)
func (q *Queries) CancelInstance(ctx context.Context, instanceUID uuid.UUID) error {
	query := `
		UPDATE calendar_events 
		SET is_cancelled = TRUE
		WHERE event_uid = $1 
		AND is_recurring_instance = TRUE
	`
	_, err := q.pool.DB().ExecContext(ctx, query, instanceUID)
	return err
}

// CountInstancesByMaster counts the number of instances for a master event
func (q *Queries) CountInstancesByMaster(ctx context.Context, masterUID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM calendar_events 
		WHERE master_event_uid = $1 
		AND is_recurring_instance = TRUE
	`
	var count int
	err := q.pool.DB().QueryRowContext(ctx, query, masterUID).Scan(&count)
	return count, err
}

// BeginTx starts a new transaction
func (q *Queries) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return q.pool.DB().BeginTx(ctx, nil)
}

// CreateEventTx creates an event within a transaction
func (q *Queries) CreateEventTx(ctx context.Context, tx *sql.Tx, event *Event) (Event, error) {
	// Validate recurrence rule if provided
	if err := rrule.ValidateRRule(event.Recurrence); err != nil {
		return Event{}, err
	}

	query := `
		--- createEventTx ---
		INSERT INTO calendar_events (
			event_uid, calendar_uid, account_id, start_ts, duration, end_ts,
			created_ts, updated_ts, recurrence, recurrence_status, recurrence_end_ts,
			exdates_ts, is_recurring_instance, master_event_uid, original_start_ts,
			is_modified, is_cancelled, metadata, timezone, local_start
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING event_uid, calendar_uid, account_id, start_ts, duration, end_ts, 
		          created_ts, updated_ts, recurrence, recurrence_status, recurrence_end_ts, 
		          exdates_ts, is_recurring_instance, master_event_uid, original_start_ts, 
		          is_modified, is_cancelled, metadata, timezone, local_start
	`

	// Marshal recurrence to JSON (empty/nil becomes SQL NULL)
	var recurrenceJSON []byte
	if !rrule.IsRRuleEmpty(event.Recurrence) {
		var err error
		recurrenceJSON, err = json.Marshal(event.Recurrence)
		if err != nil {
			return Event{}, err
		}
	}

	// exdates_ts is NOT NULL in DB, use empty array if nil
	exdatesTs := event.ExDatesTs
	if exdatesTs == nil {
		exdatesTs = []int64{}
	}

	// metadata is NOT NULL in DB, use empty object if nil
	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	var resultEvent Event
	var recurrenceJSONReturned []byte
	var recurrenceStatusReturned sql.NullString
	var recurrenceEndTsReturned sql.NullInt64
	var masterEventUIDReturned *uuid.UUID
	var originalStartTsReturned sql.NullInt64
	var timezoneReturned sql.NullString
	var localStartReturned sql.NullString

	err := tx.QueryRowContext(ctx, query,
		event.EventUID,
		event.CalendarUID,
		event.AccountID,
		event.StartTs,
		event.Duration,
		event.EndTs,
		event.CreatedTs,
		event.UpdatedTs,
		db.ToNullString(recurrenceJSON),
		db.ToNullableString(event.RecurrenceStatus),
		db.ToNullInt64(event.RecurrenceEndTs),
		pq.Array(exdatesTs),
		event.IsRecurringInstance,
		db.ToNullUUID(event.MasterEventUID),
		db.ToNullInt64(event.OriginalStartTs),
		event.IsModified,
		event.IsCancelled,
		metadata,
		db.ToNullableString(event.Timezone),
		db.ToNullableString(event.LocalStart),
	).Scan(
		&resultEvent.EventUID,
		&resultEvent.CalendarUID,
		&resultEvent.AccountID,
		&resultEvent.StartTs,
		&resultEvent.Duration,
		&resultEvent.EndTs,
		&resultEvent.CreatedTs,
		&resultEvent.UpdatedTs,
		&recurrenceJSONReturned,
		&recurrenceStatusReturned,
		&recurrenceEndTsReturned,
		pq.Array(&resultEvent.ExDatesTs),
		&resultEvent.IsRecurringInstance,
		&masterEventUIDReturned,
		&originalStartTsReturned,
		&resultEvent.IsModified,
		&resultEvent.IsCancelled,
		&resultEvent.Metadata,
		&timezoneReturned,
		&localStartReturned,
	)
	if err != nil {
		return Event{}, err
	}

	// Parse recurrence JSON
	if len(recurrenceJSONReturned) > 0 {
		if err := json.Unmarshal(recurrenceJSONReturned, &resultEvent.Recurrence); err != nil {
			return Event{}, err
		}
		if rrule.IsRRuleEmpty(resultEvent.Recurrence) {
			resultEvent.Recurrence = nil
		}
	}

	// Map nullable fields
	if recurrenceStatusReturned.Valid {
		status := RecurrenceStatus(recurrenceStatusReturned.String)
		resultEvent.RecurrenceStatus = &status
	}
	if recurrenceEndTsReturned.Valid {
		resultEvent.RecurrenceEndTs = &recurrenceEndTsReturned.Int64
	}
	resultEvent.MasterEventUID = masterEventUIDReturned
	if originalStartTsReturned.Valid {
		resultEvent.OriginalStartTs = &originalStartTsReturned.Int64
	}
	if timezoneReturned.Valid {
		resultEvent.Timezone = &timezoneReturned.String
	}
	if localStartReturned.Valid {
		resultEvent.LocalStart = &localStartReturned.String
	}

	return resultEvent, nil
}

// BulkInsertInstancesTx inserts multiple event instances within a transaction
func (q *Queries) BulkInsertInstancesTx(ctx context.Context, tx *sql.Tx, instances []*Event) error {
	if len(instances) == 0 {
		return nil
	}

	query := `
		INSERT INTO calendar_events (
			event_uid, calendar_uid, account_id, start_ts, duration, end_ts,
			created_ts, updated_ts, timezone, local_start,
			is_recurring_instance, master_event_uid, original_start_ts,
			is_modified, is_cancelled, metadata, exdates_ts
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			logger.Warn("Failed to close statement: %v", err)
		}
	}()

	for _, inst := range instances {
		exdatesTs := inst.ExDatesTs
		if exdatesTs == nil {
			exdatesTs = []int64{}
		}

		// metadata is NOT NULL in DB, use empty object if nil
		metadata := inst.Metadata
		if len(metadata) == 0 {
			metadata = json.RawMessage("{}")
		}

		_, err := stmt.ExecContext(ctx,
			inst.EventUID,
			inst.CalendarUID,
			inst.AccountID,
			inst.StartTs,
			inst.Duration,
			inst.EndTs,
			inst.CreatedTs,
			inst.UpdatedTs,
			inst.Timezone,
			inst.LocalStart,
			inst.IsRecurringInstance,
			inst.MasterEventUID,
			inst.OriginalStartTs,
			inst.IsModified,
			inst.IsCancelled,
			metadata,
			pq.Array(exdatesTs),
		)
		if err != nil {
			return fmt.Errorf("failed to insert instance: %w", err)
		}
	}

	return nil
}

// CreateEventWithInstancesTx creates a recurring event with instances within a transaction
func (q *Queries) CreateEventWithInstancesTx(
	ctx context.Context,
	tx *sql.Tx,
	event *Event,
	window GenerationWindow,
) (*Event, []*Event, error) {
	// Validate this is a recurring event
	if event.Recurrence == nil || event.Recurrence.Rule == "" {
		return nil, nil, nil
	}

	if err := rrule.ValidateRRule(event.Recurrence); err != nil {
		return nil, nil, err
	}

	// Set master event properties
	event.IsRecurringInstance = false
	event.MasterEventUID = nil
	event.OriginalStartTs = nil

	// Calculate recurrence status
	status := q.calculateRecurrenceStatus(event, window)
	event.RecurrenceStatus = &status

	// Create the master event
	createdEvent, err := q.CreateEventTx(ctx, tx, event)
	if err != nil {
		return nil, nil, err
	}
	*event = createdEvent

	// Generate instances for the window
	now := time.Now().UTC()
	windowEnd := now.Add(window.WindowDuration + window.BufferDuration)

	instances := q.generateInstances(event, event.StartTs, windowEnd.Unix())

	// Bulk insert the instances
	if len(instances) > 0 {
		if err := q.BulkInsertInstancesTx(ctx, tx, instances); err != nil {
			return nil, nil, err
		}
	}

	return event, instances, nil
}
