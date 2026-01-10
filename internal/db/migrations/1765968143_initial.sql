-- ============================================================================
-- DB Setup
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- ============================================================================
-- Accounts
-- ============================================================================
CREATE TABLE IF NOT EXISTS accounts (
    account_id TEXT PRIMARY KEY NOT NULL UNIQUE,
    created_ts BIGINT NOT NULL,
    updated_ts BIGINT NOT NULL,
    settings JSON NOT NULL,
    metadata JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_accounts_account_id ON accounts(account_id);
CREATE INDEX IF NOT EXISTS idx_accounts_metadata ON accounts USING GIN(metadata);
-- Index for timestamp
CREATE INDEX IF NOT EXISTS idx_accounts_updated_ts ON accounts(updated_ts);
CREATE INDEX IF NOT EXISTS idx_accounts_created_ts ON accounts(created_ts);
-- ============================================================================
-- Calendars
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendars (
    calendar_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    created_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    updated_ts BIGINT NOT NULL DEFAULT EXTRACT(
        EPOCH
        FROM NOW()
    )::BIGINT,
    settings JSON NOT NULL,
    metadata JSONB NOT NULL
);
-- Optimized indexes for calendars
CREATE INDEX IF NOT EXISTS idx_calendars_account_id ON calendars(account_id);
CREATE INDEX IF NOT EXISTS idx_calendars_metadata ON calendars USING GIN(metadata);
-- Composite index for common query pattern (filtering by user and calendar together)
CREATE INDEX IF NOT EXISTS idx_calendars_uid_account ON calendars(calendar_uid, account_id);
-- ============================================================================
-- Calendar Events
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendar_events (
    event_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    calendar_uid UUID NOT NULL REFERENCES calendars(calendar_uid) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    start_ts BIGINT NOT NULL,
    duration BIGINT NOT NULL,
    end_ts BIGINT NOT NULL,
    created_ts BIGINT NOT NULL,
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
-- ============================================================================
-- Reminders
-- ============================================================================
CREATE TABLE IF NOT EXISTS calendar_event_reminders (
    reminder_uid UUID PRIMARY KEY DEFAULT uuid_generate_v4() NOT NULL,
    event_uid UUID NOT NULL REFERENCES calendar_events(event_uid) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
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
-- ============================================================================
-- Views for Common Queries
-- ============================================================================
-- View for events with calendar and user info
CREATE OR REPLACE VIEW v_events_detailed AS
SELECT e.*,
    c.account_id as calendar_account_id,
    c.settings as calendar_settings,
    c.metadata as calendar_metadata
FROM calendar_events e
    JOIN calendars c ON e.calendar_uid = c.calendar_uid;
-- View for active webhooks
CREATE OR REPLACE VIEW v_active_webhooks AS
SELECT w.*
FROM webhooks w
WHERE w.is_active = true;
-- ============================================================================
-- Statistics and Monitoring Views
-- ============================================================================
CREATE OR REPLACE VIEW v_webhook_health AS
SELECT w.webhook_uid,
    w.url,
    w.is_active,
    w.failure_count,
    w.last_success_at_ts,
    w.last_failure_at_ts,
    COUNT(wd.delivery_uid) as total_deliveries,
    COUNT(wd.delivery_uid) FILTER (
        WHERE wd.http_status >= 200
            AND wd.http_status < 300
    ) as successful_deliveries,
    COUNT(wd.delivery_uid) FILTER (
        WHERE wd.http_status >= 400
            OR wd.http_status IS NULL
    ) as failed_deliveries,
    AVG(wd.response_time_ms) as avg_response_time_ms
FROM webhooks w
    LEFT JOIN webhook_deliveries wd ON w.webhook_uid = wd.webhook_uid
GROUP BY w.webhook_uid,
    w.url,
    w.is_active,
    w.failure_count,
    w.last_success_at_ts,
    w.last_failure_at_ts;