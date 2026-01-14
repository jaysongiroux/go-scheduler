# Go-Scheduler

A high-performance, feature-rich calendar and scheduling API built with Go and PostgreSQL. Provides comprehensive event management with support for recurring events, reminders, attendees, webhooks, and ICS calendar imports.

## Features

### Core Functionality
- **Calendar Management** - Create, update, and share calendars with granular permissions
- **Event Scheduling** - Full support for single and recurring events with RFC 5545 RRULE
- **Recurring Events** - Advanced recurrence patterns with instance generation and modification
- **Timezone Support** - DST-aware scheduling with IANA timezone database
- **Reminders** - Flexible reminder system with customizable delivery
- **Attendees & RSVP** - Multi-user event participation with RSVP tracking
- **Calendar Sharing** - Role-based access control for shared calendars

### Advanced Features
- **ICS Import** - Import from `.ics` files or URLs with automatic synchronization
- **Read-Only Calendars** - Subscribe to external calendars (Google Calendar, Outlook, etc.)
- **Webhook System** - Real-time event notifications with signature verification
- **Batch Operations** - Efficient bulk updates for recurring event series
- **Event Permissions** - Fine-grained access control for calendar operations

### Developer Experience
- **RESTful API** - Clean, well-documented HTTP API
- **TypeScript SDK** - Full-featured Node.js SDK with type safety
- **Comprehensive Tests** - Extensive E2E test suite with >90% coverage
- **Docker Support** - Easy deployment with Docker and Docker Compose
- **Webhook Utilities** - Built-in webhook verification and handling

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/jaysongiroux/go-scheduler.git
cd go-scheduler

# Start services
cd infra
docker-compose up -d

# The API will be available at http://localhost:8080
```

### Manual Installation

**Prerequisites:**
- Go 1.21+
- PostgreSQL 14+
- Node.js 18+ (for SDK development)

**Setup:**

```bash
# Install dependencies
go mod download

# Set environment variables
export DATABASE_URL="postgres://user:password@localhost:5432/scheduler?sslmode=disable"
export API_KEY="your-secret-api-key"
export ICS_ENCRYPTION_KEY="your-32-byte-encryption-key"

# Run database migrations
make migrate

# Build and run
make build
./bin/scheduler
```

## API Overview

### Authentication

All API requests require an API key in the header:

```bash
curl -H "api-key: your-api-key-here" http://localhost:8080/api/v1/calendars
```

### Example Requests

**Create a Calendar:**
```bash
curl -X POST http://localhost:8080/api/v1/calendars \
  -H "api-key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user123",
    "metadata": {"name": "Work Calendar"}
  }'
```

**Create an Event:**
```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "api-key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "calendar_uid": "cal-uuid",
    "account_id": "user123",
    "start_ts": 1705970000,
    "end_ts": 1705973600,
    "metadata": {"title": "Team Meeting"}
  }'
```

**Import ICS from URL:**
```bash
curl -X POST http://localhost:8080/api/v1/calendars/import/ics-link \
  -H "api-key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user123",
    "ics_url": "https://calendar.example.com/feed.ics",
    "auth_type": "none",
    "sync_interval_seconds": 86400
  }'
```

For complete API documentation, see [docs/api.md](docs/api.md).

## TypeScript SDK

Install the official Node.js SDK:

```bash
npm install go-scheduler-node-sdk
# or
yarn add go-scheduler-node-sdk
```

**Usage:**

```typescript
import { SchedulerClient } from "go-scheduler-node-sdk";

const client = new SchedulerClient({
  baseURL: "http://localhost:8080",
  headers: {
    "api-key": "your-api-key-here",
  },
});

// Create a calendar
const calendar = await client.calendars.create({
  account_id: "user123",
  metadata: { name: "My Calendar" },
});

// Create an event
const event = await client.events.create({
  calendar_uid: calendar.calendar_uid,
  account_id: "user123",
  start_ts: Math.floor(Date.now() / 1000) + 3600,
  end_ts: Math.floor(Date.now() / 1000) + 7200,
  metadata: { title: "Meeting" },
});

// Import ICS from URL
const imported = await client.calendars.importICSLink({
  accountId: "user123",
  icsUrl: "https://calendar.example.com/feed.ics",
  authType: "none",
  syncIntervalSeconds: 86400,
});
```

For SDK documentation, see [clients/node/README.md](clients/node/README.md).

## Webhook System

Subscribe to real-time event notifications:

```typescript
// Create a webhook
const webhook = await client.webhooks.create({
  url: "https://your-app.com/webhook",
  event_types: [
    "event.created",
    "event.updated",
    "calendar.synced",
    "reminder.due",
  ],
});

// Verify webhook signatures
import { WebhookUtils } from "go-scheduler-node-sdk";

const isValid = WebhookUtils.verifySignature(
  req.body, // Raw request body
  req.headers["x-webhook-signature"],
  webhook.secret
);
```

### Webhook Event Types

- **Calendar**: `created`, `updated`, `deleted`, `synced`, `resynced`
- **Event**: `created`, `updated`, `deleted`, `cancelled`, `uncancelled`
- **Reminder**: `created`, `updated`, `deleted`, `due`
- **Attendee**: `created`, `updated`, `deleted`, `rsvp_updated`
- **Member**: `invited`, `status_updated`, `role_updated`, `removed`

## ICS Calendar Integration

### Import from File

```bash
curl -X POST http://localhost:8080/api/v1/calendars/import/ics \
  -H "api-key: your-key" \
  -F "file=@calendar.ics" \
  -F "account_id=user123" \
  -F "calendar_metadata={\"name\":\"Imported Calendar\"}"
```

### Subscribe to External Calendar

```typescript
// Subscribe to a Google Calendar, Outlook Calendar, etc.
const calendar = await client.calendars.importICSLink({
  accountId: "user123",
  icsUrl: "https://calendar.google.com/calendar/ical/example/basic.ics",
  authType: "none",
  syncIntervalSeconds: 3600, // Sync every hour
  calendarMetadata: { name: "Google Calendar", source: "google" },
});

// The calendar is read-only and automatically syncs
console.log(`Syncing every ${calendar.ics_sync_interval_seconds} seconds`);

// Manually trigger a resync
const result = await client.calendars.resync(calendar.calendar_uid);
```

**Features:**
- Automatic periodic synchronization
- ETags and Last-Modified support for efficient syncing
- Basic and Bearer authentication support
- Encrypted credential storage (AES-256-GCM)
- Full replacement or partial failure handling
- Webhook notifications for sync events

## Recurring Events

Create complex recurring patterns using RFC 5545 RRULE:

```typescript
// Every Monday, Wednesday, Friday for 10 weeks
const event = await client.events.create({
  calendar_uid: "cal-uuid",
  account_id: "user123",
  start_ts: startTime,
  end_ts: endTime,
  recurrence: {
    rule: "FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=30",
  },
  metadata: { title: "Standup Meeting" },
});

// Update single instance
await client.events.update(instanceUid, {
  account_id: "user123",
  start_ts: newTime,
  scope: "single", // Only this instance
});

// Update all future instances
await client.events.update(instanceUid, {
  account_id: "user123",
  start_ts: newTime,
  scope: "future", // This and all future instances
});

// Update entire series
await client.events.update(masterUid, {
  account_id: "user123",
  start_ts: newTime,
  scope: "all", // All instances
});
```

## Architecture

### Tech Stack
- **Backend**: Go 1.21+ with standard library HTTP server
- **Database**: PostgreSQL 14+ with pgx driver
- **Migrations**: SQL migration system
- **SDK**: TypeScript with Axios
- **Testing**: Vitest for E2E tests
- **Deployment**: Docker & Docker Compose

### Project Structure

```
go-scheduler/
├── main.go                 # Application entry point
├── internal/
│   ├── api/
│   │   ├── handlers/      # HTTP request handlers
│   │   ├── middleware/    # Auth, CORS, logging
│   │   └── web/           # Response helpers
│   ├── config/            # Configuration management
│   ├── crypto/            # Encryption utilities
│   ├── db/
│   │   ├── migrations/    # SQL migrations
│   │   └── services/      # Database queries
│   ├── ics/               # ICS parser and converter
│   ├── rrule/             # Recurrence rule engine
│   └── workers/           # Background workers
│       ├── recurring.go    # Instance generation
│       ├── ics_sync.go     # ICS synchronization
│       ├── reminder_trigger.go  # Reminder delivery
│       └── webhook_dispatcher.go # Webhook delivery
├── clients/node/          # TypeScript SDK
├── e2e/                   # End-to-end tests
├── docs/                  # Documentation
│   ├── api.md            # API reference
│   └── env-vars.md       # Environment variables
└── infra/                # Docker & deployment
```

### Background Workers

1. **Recurring Event Generator** - Generates future instances for recurring events
2. **ICS Sync Worker** - Periodically syncs ICS-linked calendars
3. **Reminder Trigger** - Delivers reminders at scheduled times
4. **Webhook Dispatcher** - Delivers webhook notifications with retries

## Configuration

### Environment Variables

All configuration is done via environment variables. See [docs/env-vars.md](docs/env-vars.md) for comprehensive documentation.

**Required:**
- `API_KEY` - API authentication key
- `DATABASE_URL` - PostgreSQL connection string
- `ICS_ENCRYPTION_KEY` - 32-byte key for credential encryption (required if using ICS import)

**Optional:**
```bash
# Server
WEB_SERVER_ADDRESS=:8080
LOG_LEVEL=info

# Database
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Workers
RECURRING_GENERATION_INTERVAL=5m
ICS_SYNC_INTERVAL=1h
REMINDER_POLL_INTERVAL=30s
WEBHOOK_WORKER_CONCURRENCY=10

# Webhooks
WEBHOOK_MAX_RETRIES=3
WEBHOOK_TIMEOUT_SECONDS=30
```

### Generate Encryption Key

```bash
# Generate a secure 32-byte key for ICS_ENCRYPTION_KEY
openssl rand -base64 32 | head -c 32
```

## Development

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Node.js 18+ (for SDK and tests)
- Make

### Setup Development Environment

```bash
# Install dependencies
go mod download
cd e2e && yarn install
cd clients/node && yarn install

# Start PostgreSQL (via Docker)
docker run -d \
  --name scheduler-db \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=scheduler \
  -p 5432:5432 \
  postgres:14

# Set environment variables
export DATABASE_URL="postgres://postgres:password@localhost:5432/scheduler?sslmode=disable"
export API_KEY="test-api-key"
export ICS_ENCRYPTION_KEY="abcdefghijklmnopqrstuvwxyz123456"

# Run migrations
make migrate

# Build
make build

# Run
./bin/scheduler
```

### Running Tests

```bash
# Run E2E tests
cd e2e
yarn test

# Run specific test file
yarn test calendar

# Run with coverage
yarn test --coverage

# SDK tests
cd clients/node
yarn test
```

### Building the SDK

```bash
cd clients/node
yarn build

# Publish to npm (if you have permissions)
npm publish
```

## Database Schema

### Core Tables
- `calendars` - Calendar entities with ICS sync configuration
- `calendar_events` - Event records with recurrence support
- `calendar_event_reminders` - Reminder configurations
- `event_attendees` - Event participants and RSVP status
- `calendar_members` - Calendar sharing and permissions

### Supporting Tables
- `webhook_endpoints` - Webhook configurations
- `webhook_deliveries` - Webhook delivery history and status

See migration files in `internal/db/migrations/` for complete schema.

## Performance

### Optimization Features

- **Connection Pooling** - Configurable database connection pool
- **Row Locking** - `FOR UPDATE SKIP LOCKED` for worker concurrency
- **Batch Operations** - Bulk instance generation and webhook delivery
- **Efficient Queries** - Indexed timestamp ranges for event queries
- **Instance Caching** - Pre-generated recurring event instances
- **ETag Support** - Conditional HTTP requests for ICS sync

### Scalability

- **Horizontal Scaling** - Stateless API servers
- **Worker Isolation** - Separate worker pools per function
- **Database Indexes** - Optimized for common query patterns
- **Batch Webhooks** - Configurable batch size for high-volume events

## Security

### Authentication & Authorization

- API key authentication (SHA-256 hash comparison)
- Row-level security for calendar permissions
- Webhook signature verification (HMAC-SHA256)
- Encrypted credential storage (AES-256-GCM)

### Best Practices

- Never commit `.env` files
- Rotate API keys periodically
- Use TLS in production (`DATABASE_URL` with `sslmode=require`)
- Store `ICS_ENCRYPTION_KEY` in a secrets manager
- Validate webhook signatures before processing

## Production Deployment

### Docker

```bash
# Build image
docker build -t go-scheduler:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://..." \
  -e API_KEY="..." \
  -e ICS_ENCRYPTION_KEY="..." \
  go-scheduler:latest
```

### Docker Compose

```bash
cd infra
docker-compose up -d
```

The `docker-compose.yaml` includes:
- PostgreSQL database
- Go-scheduler API
- Automatic migrations
- Health checks

### Environment-Specific Configuration

**Production:**
```bash
LOG_LEVEL=warn
DB_MAX_OPEN_CONNS=100
WEBHOOK_WORKER_CONCURRENCY=20
ICS_SYNC_INTERVAL=30m
RECURRING_GENERATION_INTERVAL=5m
```

**Development:**
```bash
LOG_LEVEL=debug
DB_MAX_OPEN_CONNS=10
WEBHOOK_WORKER_CONCURRENCY=5
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "time": "1705968143"
}
```

### Metrics

Monitor these key metrics:
- API response times
- Database connection pool usage
- Webhook delivery success rate
- ICS sync failures
- Recurring event generation lag
- Reminder delivery latency

## API Documentation

- **[API Reference](docs/api.md)** - Complete REST API documentation
- **[Environment Variables](docs/env-vars.md)** - Configuration reference
- **[Node.js SDK](clients/node/README.md)** - TypeScript SDK documentation

## Examples

### Complete Calendar Application

```typescript
import { SchedulerClient } from "go-scheduler-node-sdk";

const client = new SchedulerClient({
  baseURL: "http://localhost:8080",
  headers: { "api-key": process.env.API_KEY },
});

async function setupCalendar() {
  // Create a calendar
  const calendar = await client.calendars.create({
    account_id: "user123",
    metadata: { name: "Team Calendar", color: "#FF5733" },
  });

  // Share with team members
  await client.calendarMembers.invite(calendar.calendar_uid, {
    members: [
      { account_id: "user456", role: "write" },
      { account_id: "user789", role: "read" },
    ],
  });

  // Create a recurring event
  const event = await client.events.create({
    calendar_uid: calendar.calendar_uid,
    account_id: "user123",
    start_ts: Math.floor(Date.now() / 1000) + 86400,
    end_ts: Math.floor(Date.now() / 1000) + 86400 + 3600,
    recurrence: {
      rule: "FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=30",
    },
    metadata: {
      title: "Daily Standup",
      location: "Conference Room A",
    },
  });

  // Add attendees
  await client.attendees.create(event.event_uid, {
    account_id: "user456",
    role: "attendee",
    metadata: { department: "engineering" },
    scope: "all", // All instances
  });

  // Add reminders
  await client.reminders.create(event.event_uid, {
    account_id: "user123",
    offset_seconds: -900, // 15 minutes before
    metadata: { method: "email" },
    scope: "all",
  });

  // Subscribe to external calendar
  const googleCal = await client.calendars.importICSLink({
    accountId: "user123",
    icsUrl: "https://calendar.google.com/calendar/ical/.../basic.ics",
    authType: "none",
    syncIntervalSeconds: 3600,
    calendarMetadata: { name: "Google Calendar", source: "google" },
  });

  return { calendar, event, googleCal };
}
```

## Support

- **Documentation**: [docs/](docs/)
- **Examples**: [e2e/tests/](e2e/tests/)
