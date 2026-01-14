# Environment Variables

This document describes all environment variables used by the Go Scheduler application.

## Table of Contents

- [Required Variables](#required-variables)
- [Database Configuration](#database-configuration)
- [Server Configuration](#server-configuration)
- [Recurring Event Generation](#recurring-event-generation)
- [Webhook Delivery](#webhook-delivery)
- [Reminder Trigger Worker](#reminder-trigger-worker)
- [ICS Calendar Sync](#ics-calendar-sync)
- [API Configuration](#api-configuration)

---

## Required Variables

These variables **must** be set for the application to start.

### `API_KEY`

**Required:** Yes  
**Type:** String  
**Description:** API key used for authenticating requests. This is hashed using SHA-256 and stored as `APIKeyHash`.  
**Example:** `your-secret-api-key-here`

---

### `DATABASE_URL`

**Required:** Yes  
**Type:** String (PostgreSQL connection URL)  
**Description:** PostgreSQL database connection string.  
**Format:** `postgres://username:password@host:port/database?sslmode=disable`  
**Example:** `postgres://scheduler:password@localhost:5432/scheduler?sslmode=disable`

---

### `ICS_ENCRYPTION_KEY`

**Required:** Yes  
**Type:** String (32 bytes)  
**Description:** Encryption key for securing ICS calendar credentials (basic auth passwords, bearer tokens). Must be exactly 32 bytes for AES-256 encryption.  
**Example:** `abcdefghijklmnopqrstuvwxyz123456` (32 characters)  
**Note:** Generate a secure random 32-byte string. Do not use a simple password.

```bash
# Generate a secure key:
openssl rand -base64 32 | head -c 32
```

---

## Database Configuration

### `DB_MAX_OPEN_CONNS`

**Required:** No  
**Type:** Integer  
**Default:** `25`  
**Description:** Maximum number of open connections to the database.  
**Example:** `DB_MAX_OPEN_CONNS=50`

---

### `DB_MAX_IDLE_CONNS`

**Required:** No  
**Type:** Integer  
**Default:** `5`  
**Description:** Maximum number of idle connections in the database pool.  
**Example:** `DB_MAX_IDLE_CONNS=10`

---

## Server Configuration

### `WEB_SERVER_ADDRESS`

**Required:** No  
**Type:** String (host:port)  
**Default:** `:8080`  
**Description:** Address and port for the HTTP server to listen on.  
**Example:** `WEB_SERVER_ADDRESS=0.0.0.0:8080`

---

### `LOG_LEVEL`

**Required:** No  
**Type:** String  
**Default:** `info`  
**Options:** `debug`, `info`, `warn`, `error`, `fatal`  
**Description:** Logging level for the application.  
**Example:** `LOG_LEVEL=debug`

---

## Recurring Event Generation

These variables control the background worker that generates instances for recurring events.

### `RECURRING_GENERATION_INTERVAL`

**Required:** No  
**Type:** Duration  
**Default:** `5m` (5 minutes)  
**Description:** How often the recurring event generation worker runs to check for events needing instance generation.  
**Format:** Go duration string (e.g., `30s`, `5m`, `1h`)  
**Example:** `RECURRING_GENERATION_INTERVAL=10m`

---

### `GENERATION_WINDOW`

**Required:** No  
**Type:** Duration  
**Default:** `8760h` (365 days / 1 year)  
**Description:** How far into the future to generate recurring event instances.  
**Format:** Go duration string  
**Example:** `GENERATION_WINDOW=4380h` (6 months)

---

### `GENERATION_BUFFER`

**Required:** No  
**Type:** Duration  
**Default:** `168h` (7 days)  
**Description:** Extra time buffer added to the generation window to prevent gaps in recurring event instances.  
**Format:** Go duration string  
**Example:** `GENERATION_BUFFER=336h` (14 days)

---

## Webhook Delivery

These variables control the background worker that delivers webhooks.

### `WEBHOOK_WORKER_CONCURRENCY`

**Required:** No  
**Type:** Integer  
**Default:** `5`  
**Description:** Number of concurrent goroutines for processing webhook deliveries.  
**Example:** `WEBHOOK_WORKER_CONCURRENCY=10`

---

### `WEBHOOK_BASE_RETRY_DELAY`

**Required:** No  
**Type:** Duration  
**Default:** `1s` (1 second)  
**Description:** Base delay for exponential backoff when retrying failed webhook deliveries.  
**Format:** Go duration string  
**Example:** `WEBHOOK_BASE_RETRY_DELAY=2s`  
**Note:** Actual retry delay = `base_delay * (2 ^ attempt_number)`

---

### `WEBHOOK_MAX_RETRIES`

**Required:** No  
**Type:** Integer  
**Default:** `3`  
**Description:** Maximum number of retry attempts for failed webhook deliveries.  
**Example:** `WEBHOOK_MAX_RETRIES=5`

---

### `WEBHOOK_TIMEOUT_SECONDS`

**Required:** No  
**Type:** Integer  
**Default:** `30` (seconds)  
**Description:** HTTP timeout for webhook delivery requests.  
**Example:** `WEBHOOK_TIMEOUT_SECONDS=60`

---

### `WEBHOOK_MAX_BATCH_SIZE`

**Required:** No  
**Type:** Integer  
**Default:** `100`  
**Description:** Maximum number of events to batch in a single webhook delivery (for batch webhooks like `event.created`).  
**Example:** `WEBHOOK_MAX_BATCH_SIZE=50`

---

## Reminder Trigger Worker

These variables control the background worker that triggers reminders.

### `REMINDER_POLL_INTERVAL`

**Required:** No  
**Type:** Duration  
**Default:** `30s` (30 seconds)  
**Description:** How often the reminder trigger worker polls for reminders that need to be sent.  
**Format:** Go duration string  
**Example:** `REMINDER_POLL_INTERVAL=1m`

---

### `REMINDER_BATCH_SIZE`

**Required:** No  
**Type:** Integer  
**Default:** `100`  
**Description:** Maximum number of reminders to process in a single batch.  
**Example:** `REMINDER_BATCH_SIZE=200`

---

## ICS Calendar Sync

These variables control the background worker that syncs ICS calendars imported via URL.

### `ICS_SYNC_INTERVAL`

**Required:** No  
**Type:** Duration  
**Default:** `1h` (1 hour)  
**Description:** How often the ICS sync worker runs to check for calendars that need synchronization.  
**Format:** Go duration string  
**Example:** `ICS_SYNC_INTERVAL=30m`  
**Note:** Individual calendars have their own sync intervals (default 24 hours). This controls how often the worker checks for calendars needing sync.

---

### `ICS_SYNC_BATCH_SIZE`

**Required:** No  
**Type:** Integer  
**Default:** `100`  
**Description:** Maximum number of calendars to process in a single sync batch.  
**Example:** `ICS_SYNC_BATCH_SIZE=50`

---

### `ICS_REQUEST_TIMEOUT`

**Required:** No  
**Type:** Duration  
**Default:** `30s` (30 seconds)  
**Description:** HTTP timeout when fetching ICS calendar feeds from remote URLs.  
**Format:** Go duration string  
**Example:** `ICS_REQUEST_TIMEOUT=60s`

---

## API Configuration

### `DEFAULT_PAGE_SIZE`

**Required:** No  
**Type:** Integer  
**Default:** `50`  
**Description:** Default page size for paginated API responses when no `limit` parameter is provided.  
**Example:** `DEFAULT_PAGE_SIZE=100`

---

## Example Configuration

Here's a complete example `.env` file with all variables:

```bash
# Required
API_KEY=your-secret-api-key-change-this
DATABASE_URL=postgres://scheduler:password@localhost:5432/scheduler?sslmode=disable
ICS_ENCRYPTION_KEY=abcdefghijklmnopqrstuvwxyz123456

# Database (Optional)
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Server (Optional)
WEB_SERVER_ADDRESS=:8080
LOG_LEVEL=info

# Recurring Events (Optional)
RECURRING_GENERATION_INTERVAL=5m
GENERATION_WINDOW=8760h
GENERATION_BUFFER=168h

# Webhooks (Optional)
WEBHOOK_WORKER_CONCURRENCY=5
WEBHOOK_BASE_RETRY_DELAY=1s
WEBHOOK_MAX_RETRIES=3
WEBHOOK_TIMEOUT_SECONDS=30
WEBHOOK_MAX_BATCH_SIZE=100

# Reminders (Optional)
REMINDER_POLL_INTERVAL=30s
REMINDER_BATCH_SIZE=100

# ICS Sync (Optional)
ICS_SYNC_INTERVAL=1h
ICS_SYNC_BATCH_SIZE=100
ICS_REQUEST_TIMEOUT=30s

# API (Optional)
DEFAULT_PAGE_SIZE=50
```

---

## Duration Format

Variables marked as "Duration" use Go's duration string format:

- `ns` - nanoseconds
- `us` or `µs` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

**Examples:**
- `30s` = 30 seconds
- `5m` = 5 minutes
- `1h30m` = 1 hour 30 minutes
- `24h` = 24 hours

---

## Security Considerations

1. **Never commit `.env` files to version control** - Add `.env` to your `.gitignore`
2. **Use strong, random values** for `API_KEY` and `ICS_ENCRYPTION_KEY`
3. **Rotate keys periodically** - Especially the API key and encryption key
4. **Use SSL/TLS in production** - Set `sslmode=require` in `DATABASE_URL`
5. **Restrict database user permissions** - Only grant necessary privileges
6. **Store sensitive values in secrets management** - Use systems like HashiCorp Vault, AWS Secrets Manager, or Kubernetes Secrets in production

---

## Production Recommendations

### Database
- Set `DB_MAX_OPEN_CONNS` based on your database server capacity (typically 20-100)
- Set `DB_MAX_IDLE_CONNS` to ~25% of max open connections
- Use connection pooling and monitor connection usage

### Webhooks
- Increase `WEBHOOK_WORKER_CONCURRENCY` for high-volume webhook deliveries (10-20)
- Adjust `WEBHOOK_TIMEOUT_SECONDS` based on your webhook endpoint response times
- Monitor failed deliveries and adjust `WEBHOOK_MAX_RETRIES` accordingly

### ICS Sync
- Adjust `ICS_SYNC_INTERVAL` based on your user base size and sync frequency needs
- Increase `ICS_REQUEST_TIMEOUT` if fetching from slow ICS feeds
- Monitor sync failures and adjust timeouts accordingly

### Workers
- Lower `REMINDER_POLL_INTERVAL` for more real-time reminder delivery (15s-30s)
- Lower `RECURRING_GENERATION_INTERVAL` to ensure timely instance generation (3m-5m)
- Balance worker intervals with database load

### Logging
- Use `LOG_LEVEL=info` or `LOG_LEVEL=warn` in production
- Use `LOG_LEVEL=debug` only for troubleshooting (generates significant log volume)
