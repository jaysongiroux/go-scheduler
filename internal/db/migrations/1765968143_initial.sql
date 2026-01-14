-- ============================================================================
-- DB Setup
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- ============================================================================
-- Calendars
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendars (
    calendar_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    account_id TEXT NOT NULL,
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    updated_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    settings JSON NOT NULL,
    metadata JSONB NOT NULL,
    -- ICS import fields
    ics_url TEXT,
    ics_auth_type TEXT CHECK (ics_auth_type IN ('none', 'basic', 'bearer')),
    ics_auth_credentials BYTEA,
    ics_last_sync_ts BIGINT,
    ics_last_sync_status TEXT CHECK (
        ics_last_sync_status IN ('success', 'stale', 'failed')
    ),
    ics_sync_interval_seconds INT DEFAULT 86400,
    ics_error_message TEXT,
    ics_last_etag TEXT,
    ics_last_modified TEXT,
    is_read_only BOOLEAN DEFAULT FALSE NOT NULL,
    sync_on_partial_failure BOOLEAN DEFAULT TRUE NOT NULL
);
-- Optimized indexes for calendars
CREATE INDEX IF NOT EXISTS idx_calendars_account_id ON calendars(account_id);
CREATE INDEX IF NOT EXISTS idx_calendars_metadata ON calendars USING GIN(metadata);
-- ICS import indexes
CREATE INDEX IF NOT EXISTS idx_calendars_ics_url ON calendars(ics_url)
WHERE ics_url IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_calendars_next_sync ON calendars(ics_last_sync_ts, ics_sync_interval_seconds)
WHERE ics_url IS NOT NULL;
-- ============================================================================
-- Calendar Members
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendar_members (
    account_id TEXT NOT NULL,
    calendar_uid UUID NOT NULL REFERENCES calendars(calendar_uid) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed')),
    role TEXT NOT NULL CHECK (role IN ('read', 'write')) DEFAULT 'write',
    invited_by TEXT NOT NULL,
    invited_at_ts BIGINT NOT NULL,
    updated_ts BIGINT NOT NULL,
    PRIMARY KEY (account_id, calendar_uid)
);
-- Indexes for calendar_members
CREATE INDEX IF NOT EXISTS idx_calendar_members_calendar ON calendar_members(calendar_uid);
CREATE INDEX IF NOT EXISTS idx_calendar_members_account ON calendar_members(account_id);
CREATE INDEX IF NOT EXISTS idx_calendar_members_status ON calendar_members(calendar_uid, status);
-- ============================================================================
-- Calendar Events
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendar_events (
    event_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    calendar_uid UUID NOT NULL REFERENCES calendars(calendar_uid) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    start_ts BIGINT NOT NULL,
    duration BIGINT NOT NULL,
    end_ts BIGINT NOT NULL,
    created_ts BIGINT NOT NULL,
    timezone TEXT,
    local_start TEXT,
    updated_ts BIGINT NOT NULL,
    -- Recurrence tracking (master event fields)
    recurrence JSON,
    recurrence_status TEXT CHECK (recurrence_status IN ('active', 'inactive')),
    recurrence_end_ts BIGINT,
    exdates_ts BIGINT [] NOT NULL DEFAULT '{}',
    -- Instance tracking fields
    is_recurring_instance BOOLEAN NOT NULL DEFAULT FALSE,
    master_event_uid UUID REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    original_start_ts BIGINT,
    -- State tracking
    is_modified BOOLEAN NOT NULL DEFAULT FALSE,
    is_cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    -- Metadata
    metadata JSONB NOT NULL,
    -- Constraints: instances must have master, non-instances must not
    CONSTRAINT instances_have_master CHECK (
        (
            is_recurring_instance = FALSE
            AND master_event_uid IS NULL
        )
        OR (
            is_recurring_instance = TRUE
            AND master_event_uid IS NOT NULL
        )
    )
);
-- Optimized indexes for calendar_events
CREATE INDEX IF NOT EXISTS idx_events_calendar_uid ON calendar_events(calendar_uid);
CREATE INDEX IF NOT EXISTS idx_events_account_id ON calendar_events(account_id);
CREATE INDEX IF NOT EXISTS idx_events_metadata ON calendar_events USING GIN(metadata);
-- Time-based query optimization
CREATE INDEX IF NOT EXISTS idx_events_start_ts ON calendar_events(start_ts);
CREATE INDEX IF NOT EXISTS idx_events_end_ts ON calendar_events(end_ts);
CREATE INDEX IF NOT EXISTS idx_events_time_range ON calendar_events(start_ts, end_ts);
-- BRIN index for time-series optimization (efficient for large datasets with sequential time values)
CREATE INDEX IF NOT EXISTS idx_events_start_ts_brin ON calendar_events USING BRIN(start_ts);
-- Composite index for common query pattern (events for a calendar in a time range)
CREATE INDEX IF NOT EXISTS idx_events_calendar_time ON calendar_events(calendar_uid, start_ts, end_ts)
WHERE is_cancelled = FALSE;
-- Index for finding events with recurrence
CREATE INDEX IF NOT EXISTS idx_events_recurrence ON calendar_events(calendar_uid)
WHERE recurrence IS NOT NULL;
-- Index for updated timestamp (useful for sync queries)
CREATE INDEX IF NOT EXISTS idx_events_updated_ts ON calendar_events(updated_ts);
-- Prevent duplicate instances (race protection for worker)
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_instance ON calendar_events(master_event_uid, original_start_ts)
WHERE is_recurring_instance = TRUE;
-- Worker queries: find active recurring events needing generation
CREATE INDEX IF NOT EXISTS idx_active_recurrence ON calendar_events(recurrence_status)
WHERE recurrence_status = 'active'
    AND is_recurring_instance = FALSE;
-- Find all instances of a master event
CREATE INDEX IF NOT EXISTS idx_instances_by_master ON calendar_events(master_event_uid, start_ts)
WHERE is_recurring_instance = TRUE;
-- Account-level queries with cancellation filter
CREATE INDEX IF NOT EXISTS idx_events_account_time ON calendar_events(account_id, start_ts, end_ts)
WHERE is_cancelled = FALSE;
-- Add index for timezone queries (optional, for analytics)
CREATE INDEX IF NOT EXISTS idx_events_timezone ON calendar_events(timezone)
WHERE timezone IS NOT NULL;
-- Add comments for documentation
COMMENT ON COLUMN calendar_events.timezone IS 'IANA timezone string (e.g., America/New_York) for DST-aware scheduling';
COMMENT ON COLUMN calendar_events.local_start IS 'Wall-clock time in RFC3339 format without offset, preserves local time intent';
-- ============================================================================
-- Calendar event attendees
-- ============================================================================
CREATE TABLE IF NOT EXISTS event_attendees (
    attendee_uid UUID PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
    event_uid UUID NOT NULL REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    master_event_uid UUID REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    attendee_group_id UUID,
    role TEXT NOT NULL CHECK (role IN ('organizer', 'attendee')),
    rsvp_status TEXT NOT NULL DEFAULT 'pending' CHECK (
        rsvp_status IN ('pending', 'accepted', 'declined', 'tentative')
    ),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    ),
    updated_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    ),
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    archived_ts BIGINT,
    CONSTRAINT unique_event_attendee UNIQUE (event_uid, account_id)
);
-- Indexes for query optimization
CREATE INDEX IF NOT EXISTS idx_attendee_event ON event_attendees(event_uid, archived);
CREATE INDEX IF NOT EXISTS idx_attendee_account ON event_attendees(account_id, archived)
WHERE archived = FALSE;
CREATE INDEX IF NOT EXISTS idx_attendee_group ON event_attendees(attendee_group_id, archived)
WHERE attendee_group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_attendee_master ON event_attendees(master_event_uid, archived)
WHERE master_event_uid IS NOT NULL
    AND archived = FALSE;
CREATE INDEX IF NOT EXISTS idx_attendee_rsvp ON event_attendees(event_uid, rsvp_status)
WHERE archived = FALSE;
CREATE INDEX IF NOT EXISTS idx_attendee_role ON event_attendees(event_uid, role)
WHERE archived = FALSE;
-- For worker copy operations
CREATE INDEX IF NOT EXISTS idx_attendee_worker_copy ON event_attendees(event_uid)
WHERE archived = FALSE;
-- For invited events query (join optimization)
CREATE INDEX IF NOT EXISTS idx_attendee_account_time ON event_attendees(account_id, event_uid)
WHERE archived = FALSE;
-- ============================================================================
-- Reminders
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendar_event_reminders (
    reminder_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    event_uid UUID NOT NULL REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    master_event_uid UUID REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    reminder_group_id UUID,
    offset_seconds BIGINT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    is_delivered BOOLEAN NOT NULL DEFAULT FALSE,
    delivered_ts BIGINT,
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    ),
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    archived_ts BIGINT,
    CONSTRAINT check_offset_negative CHECK (offset_seconds <= 0)
);
CREATE INDEX IF NOT EXISTS idx_reminder_event ON calendar_event_reminders(event_uid, archived);
CREATE INDEX IF NOT EXISTS idx_reminder_group ON calendar_event_reminders(reminder_group_id, archived)
WHERE reminder_group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_reminder_master ON calendar_event_reminders(master_event_uid, archived)
WHERE master_event_uid IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_reminder_delivery ON calendar_event_reminders(account_id, is_delivered, archived)
WHERE archived = FALSE;
CREATE INDEX IF NOT EXISTS idx_reminder_master_archived ON calendar_event_reminders(master_event_uid)
WHERE archived = FALSE;
-- ============================================================================
-- Webhooks
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhooks (
    webhook_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(64) NOT NULL,
    -- HMAC secret for signature verification
    event_types TEXT [] NOT NULL,
    -- ['event.reminder', 'event.created', 'event.updated', 'event.deleted']
    is_active BOOLEAN DEFAULT true NOT NULL,
    retry_count INT DEFAULT 3 NOT NULL,
    timeout_seconds INT DEFAULT 10 NOT NULL,
    last_triggered_at_ts BIGINT,
    last_success_at_ts BIGINT,
    last_failure_at_ts BIGINT,
    failure_count INT DEFAULT 0 NOT NULL,
    created_ts BIGINT NOT NULL,
    updated_ts BIGINT NOT NULL,
    CONSTRAINT check_retry_count CHECK (
        retry_count >= 0
        AND retry_count <= 10
    ),
    CONSTRAINT check_timeout CHECK (
        timeout_seconds > 0
        AND timeout_seconds <= 60
    )
);
-- Indexes for webhooks
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(is_active)
WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_webhooks_event_types ON webhooks USING GIN(event_types);
-- ============================================================================
-- Webhook Jobs (Delivery Queue)
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_jobs (
    job_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    webhook_uid UUID NOT NULL REFERENCES webhooks(webhook_uid) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    event_start_ts BIGINT,
    -- For staleness check: job is stale if scheduled_at > event_start_ts
    scheduled_at_ts BIGINT NOT NULL,
    next_retry_ts BIGINT NOT NULL,
    attempt_number INT DEFAULT 0 NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'completed', 'failed', 'stale')
    ),
    error_message TEXT,
    created_ts BIGINT NOT NULL
);
-- Indexes for webhook_jobs
CREATE INDEX IF NOT EXISTS idx_webhook_jobs_pending ON webhook_jobs(next_retry_ts)
WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_webhook_jobs_webhook ON webhook_jobs(webhook_uid);
CREATE INDEX IF NOT EXISTS idx_webhook_jobs_status ON webhook_jobs(status);
CREATE INDEX IF NOT EXISTS idx_webhook_jobs_created ON webhook_jobs(created_ts);
-- ============================================================================
-- Webhook Deliveries (Audit Log)
-- ============================================================================
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    delivery_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    webhook_uid UUID NOT NULL REFERENCES webhooks(webhook_uid) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    http_status INT,
    error_message TEXT,
    response_body TEXT,
    response_time_ms INT,
    attempt_number INT DEFAULT 1 NOT NULL,
    delivered_at_ts BIGINT NOT NULL
);
-- Indexes for webhook deliveries
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook ON webhook_deliveries(webhook_uid);
CREATE INDEX IF NOT EXISTS idx_deliveries_delivered_at_ts ON webhook_deliveries(delivered_at_ts);
CREATE INDEX IF NOT EXISTS idx_deliveries_status ON webhook_deliveries(http_status);
CREATE INDEX IF NOT EXISTS idx_deliveries_event_type ON webhook_deliveries(event_type);
-- Composite index for querying recent deliveries by webhook
CREATE INDEX IF NOT EXISTS idx_deliveries_webhook_time ON webhook_deliveries(webhook_uid, delivered_at_ts DESC);
-- Index for reminder.triggered event type
CREATE INDEX IF NOT EXISTS idx_deliveries_reminder_type ON webhook_deliveries(event_type)
WHERE event_type = 'reminder.triggered';
-- ============================================================================
-- Views
-- ============================================================================
-- View to track which reminders have been delivered via webhooks
CREATE OR REPLACE VIEW v_delivered_reminders AS
SELECT wd.delivery_uid,
    wd.webhook_uid,
    wd.delivered_at_ts,
    wd.http_status,
    wd.payload->'event'->>'event_uid' as event_uid,
    wd.payload->'reminder'->>'reminder_uid' as reminder_uid,
    wd.payload->'reminder' as reminder_data,
    wd.payload->'event' as event_data
FROM webhook_deliveries wd
WHERE wd.event_type = 'reminder.triggered';
-- ============================================================================
-- Functions and Triggers
-- ============================================================================
-- Function to automatically set updated timestamp
CREATE OR REPLACE FUNCTION set_updated_timestamp() RETURNS TRIGGER AS $$ BEGIN NEW.updated_ts = EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- Trigger for calendar_events
DROP TRIGGER IF EXISTS trigger_events_updated ON calendar_events;
CREATE TRIGGER trigger_events_updated BEFORE
UPDATE ON calendar_events FOR EACH ROW EXECUTE FUNCTION set_updated_timestamp();
-- Trigger for webhooks
DROP TRIGGER IF EXISTS trigger_webhooks_updated ON webhooks;
CREATE TRIGGER trigger_webhooks_updated BEFORE
UPDATE ON webhooks FOR EACH ROW EXECUTE FUNCTION set_updated_timestamp();
-- Function to automatically set created timestamp on insert
CREATE OR REPLACE FUNCTION set_created_timestamp() RETURNS TRIGGER AS $$ BEGIN IF NEW.created_ts IS NULL
    OR NEW.created_ts = 0 THEN NEW.created_ts = EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT;
END IF;
IF NEW.updated_ts IS NULL
OR NEW.updated_ts = 0 THEN NEW.updated_ts = NEW.created_ts;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- Trigger for calendar_events creation
DROP TRIGGER IF EXISTS trigger_events_created ON calendar_events;
CREATE TRIGGER trigger_events_created BEFORE
INSERT ON calendar_events FOR EACH ROW EXECUTE FUNCTION set_created_timestamp();
-- Trigger for webhooks creation
DROP TRIGGER IF EXISTS trigger_webhooks_created ON webhooks;
CREATE TRIGGER trigger_webhooks_created BEFORE
INSERT ON webhooks FOR EACH ROW EXECUTE FUNCTION set_created_timestamp();
-- Function to increment webhook failure count
CREATE OR REPLACE FUNCTION increment_webhook_failure(webhook_uuid UUID) RETURNS void AS $$ BEGIN
UPDATE webhooks
SET failure_count = failure_count + 1,
    last_failure_at_ts = EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    -- Auto-disable webhook after 10 consecutive failures
    is_active = CASE
        WHEN failure_count + 1 >= 10 THEN false
        ELSE is_active
    END
WHERE webhook_uid = webhook_uuid;
END;
$$ LANGUAGE plpgsql;
-- 
-- Function to reset webhook failure count on success
-- 
CREATE OR REPLACE FUNCTION reset_webhook_failure(webhook_uuid UUID) RETURNS void AS $$ BEGIN
UPDATE webhooks
SET failure_count = 0,
    last_success_at_ts = EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    last_triggered_at_ts = EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT
WHERE webhook_uid = webhook_uuid;
END;
$$ LANGUAGE plpgsql;
-- Trigger to auto-update updated_ts on row changes
DROP TRIGGER IF EXISTS trigger_attendees_updated ON event_attendees;
CREATE TRIGGER trigger_attendees_updated BEFORE
UPDATE ON event_attendees FOR EACH ROW EXECUTE FUNCTION set_updated_timestamp();