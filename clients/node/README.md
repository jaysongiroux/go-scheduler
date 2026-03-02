# Go-Scheduler TypeScript SDK

TypeScript SDK for interacting with the go-scheduler REST API.

## Installation

```bash
yarn install go-scheduler-node-sdk
```

or

```bash
yarn add go-scheduler-node-sdk
```

## Quick Start

```typescript
import { SchedulerClient } from "go-scheduler-node-sdk";

const client = new SchedulerClient({
  baseURL: "http://localhost:8080",
  timeout: 30000,
  headers: {
    "api-key": "your-token-here"
  }
});

// Check service health
const health = await client.healthCheck();
console.log(health);
```

## Usage Examples

### Calendars

```typescript
// Create a calendar
const calendar = await client.calendars.create({
  account_id: "user123",
  name: "My Calendar",
  description: "Personal calendar",
  timezone: "America/New_York",
  metadata: { color: "blue" }
});

// Get a calendar
const calendar = await client.calendars.get("calendar-uid");

// List calendars
const calendars = await client.calendars.list(50, 0);

// Update a calendar
const updated = await client.calendars.update("calendar-uid", {
  name: "Updated Calendar Name",
  description: "New description"
});

// Delete a calendar
await client.calendars.delete("calendar-uid");
```

### Events

```typescript
// Create a single event
const event = await client.events.create({
  calendar_uid: "calendar-uid",
  account_id: "user123",
  start_ts: Math.floor(Date.now() / 1000) + 3600,
  end_ts: Math.floor(Date.now() / 1000) + 7200,
  timezone: "America/New_York",
  metadata: {
    title: "Team Meeting",
    description: "Quarterly planning"
  }
});

// Create a recurring event
const recurringEvent = await client.events.create({
  calendar_uid: "calendar-uid",
  account_id: "user123",
  start_ts: Math.floor(Date.now() / 1000) + 3600,
  end_ts: Math.floor(Date.now() / 1000) + 7200,
  recurrence: {
    rule: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
    count: 10
  },
  metadata: {
    title: "Daily Standup"
  }
});

// Get an event
const event = await client.events.get("event-uid");

// Get events for multiple calendars
const events = await client.events.getCalendarEvents({
  calendar_uids: ["cal-1", "cal-2"],
  start_ts: Math.floor(Date.now() / 1000),
  end_ts: Math.floor(Date.now() / 1000) + 86400 * 7
});

// Update an event (single instance)
const updated = await client.events.update("event-uid", {
  account_id: "user123",
  start_ts: newStartTime,
  end_ts: newEndTime,
  scope: "single"
});

// Update all instances of a recurring event
await client.events.update("event-uid", {
  account_id: "user123",
  start_ts: newStartTime,
  end_ts: newEndTime,
  scope: "all"
});

// Toggle cancelled status
await client.events.toggleCancelled("event-uid");

// Delete an event
await client.events.delete("event-uid", "user123", "single");

// Transfer event ownership
await client.events.transferOwnership(
  "event-uid",
  "new-owner-account-id",
  "new-calendar-uid",
  "all"
);
```

### Reminders

```typescript
// Create a reminder (30 minutes before event)
const reminder = await client.reminders.create("event-uid", {
  offset_seconds: -1800,
  account_id: "user123",
  metadata: { method: "email" },
  scope: "single"
});

// List reminders for an event
const reminders = await client.reminders.list("event-uid", "user123");

// Update a reminder
const updated = await client.reminders.update("event-uid", "reminder-uid", {
  offset_seconds: -3600,
  metadata: { method: "push" },
  scope: "single"
});

// Delete a reminder
await client.reminders.delete("event-uid", "reminder-uid", "single");
```

### Webhooks

```typescript
// Create a webhook
const webhook = await client.webhooks.create({
  url: "https://example.com/webhook",
  event_types: ["event.created", "event.updated", "event.deleted"],
  retry_count: 3,
  timeout_seconds: 30
});

// Get a webhook
const webhook = await client.webhooks.get("webhook-uid");

// List webhooks
const webhooks = await client.webhooks.list(50, 0);

// Update a webhook
const updated = await client.webhooks.update("webhook-uid", {
  url: "https://example.com/new-webhook",
  is_active: true
});

// Delete a webhook
await client.webhooks.delete("webhook-uid");

// Get webhook deliveries
const deliveries = await client.webhooks.getDeliveries("webhook-uid", 50, 0);
```

### Attendees

```typescript
// Add an attendee to an event
const attendee = await client.attendees.create("event-uid", {
  account_id: "user456",
  role: "attendee",
  metadata: { department: "engineering" },
  scope: "single"
});

// List attendees for an event
const attendees = await client.attendees.list("event-uid");

// Get a specific attendee
const attendee = await client.attendees.get("event-uid", "user456");

// Update an attendee
const updated = await client.attendees.update("event-uid", "user456", {
  role: "organizer",
  scope: "single"
});

// Update RSVP status
await client.attendees.updateRSVP("event-uid", "user456", {
  rsvp_status: "accepted",
  scope: "single"
});

// Remove an attendee
await client.attendees.delete("event-uid", "user456", "single");

// Get events for an account (where they are an attendee)
const events = await client.attendees.getAccountEvents(
  "user456",
  Math.floor(Date.now() / 1000),
  Math.floor(Date.now() / 1000) + 86400 * 30,
  "attendee",
  "accepted"
);
```

### Calendar Members

```typescript
// Invite members to a calendar
await client.calendarMembers.invite("calendar-uid", {
  members: [
    { account_id: "user1", role: "read" },
    { account_id: "user2", role: "write" }
  ]
});

// List calendar members
const members = await client.calendarMembers.list("calendar-uid");

// Update a member's role
const updated = await client.calendarMembers.update("calendar-uid", "user1", {
  role: "write",
  status: "confirmed"
});

// Remove a member
await client.calendarMembers.remove("calendar-uid", "user1");
```

### ICS Import

```typescript
// Import from ICS file
const formData = new FormData();
formData.append("file", icsFileBlob);
formData.append("account_id", "user123");
formData.append("calendar_metadata", JSON.stringify({ name: "Imported Calendar" }));

const result = await client.calendars.importICS(formData);
console.log(`Imported ${result.summary.imported_events} events`);

// Import from ICS URL (with automatic sync)
const syncedCalendar = await client.calendars.importICSLink({
  accountId: "user123",
  icsUrl: "https://calendar.google.com/calendar/ical/example/basic.ics",
  authType: "none", // or "basic" or "bearer"
  syncIntervalSeconds: 86400, // 24 hours
  calendarMetadata: { name: "External Calendar", source: "google" },
});

// Manually trigger a resync
const resyncResult = await client.calendars.resync("calendar-uid");
console.log(`Resynced ${resyncResult.imported_events} events`);
```

### Webhook Utilities

The SDK includes utilities for verifying and handling webhook payloads:

```typescript
import {
  WebhookUtils,
  createWebhookVerificationMiddleware,
  WebhookPayload,
  isEventWebhook,
  isCalendarWebhook,
} from "go-scheduler-node-sdk";

// Verify a webhook signature
const rawBody = req.body; // Raw request body as string or Buffer
const signature = req.headers["x-webhook-signature"];
const secret = "your-webhook-secret";

const verification = WebhookUtils.verifySignature(rawBody, signature, secret);
if (verification.valid) {
  const payload = JSON.parse(rawBody) as WebhookPayload;
  console.log("Valid webhook:", payload.event_type);
} else {
  console.error("Invalid webhook:", verification.error);
}

// Parse and verify in one step
const result = WebhookUtils.parseAndVerify(rawBody, signature, secret);
if (result) {
  console.log("Webhook data:", result.payload.data);
}

// Verify with timestamp freshness check
const verification = WebhookUtils.verifyWithTimestamp(
  rawBody,
  signature,
  secret,
  300 // 5 minute tolerance
);

// Type guards for webhook payloads
if (isEventWebhook(payload)) {
  // TypeScript knows this is an event webhook
  console.log("Event UID:", payload.data.event_uid);
}

if (isCalendarWebhook(payload)) {
  // TypeScript knows this is a calendar webhook
  console.log("Calendar UID:", payload.data.calendar_uid);
}

// Express middleware for automatic verification
import express from "express";

const app = express();

app.use(express.json({
  verify: (req, res, buf) => {
    req.rawBody = buf.toString("utf8");
  },
}));

app.post(
  "/webhook",
  createWebhookVerificationMiddleware("your-webhook-secret", {
    rawBodyKey: "rawBody",
    toleranceSeconds: 300,
  }),
  (req, res) => {
    // Webhook is verified and parsed
    const payload = req.webhookPayload as WebhookPayload;
    console.log("Received:", payload.event_type);
    res.json({ received: true });
  }
);
```

#### Webhook Utility Functions

- **`verifySignature(payload, signature, secret)`** - Verify HMAC-SHA256 signature
- **`computeSignature(payload, secret)`** - Compute signature for testing
- **`parseAndVerify(rawPayload, signature, secret)`** - Parse and verify in one step
- **`validatePayloadStructure(payload)`** - Validate webhook payload structure
- **`isTimestampFresh(timestamp, toleranceSeconds)`** - Check timestamp freshness
- **`verifyWithTimestamp(rawPayload, signature, secret, toleranceSeconds)`** - Verify signature and timestamp
- **`extractSignature(headers)`** - Extract signature from headers
- **`createWebhookVerificationMiddleware(secret, options)`** - Express middleware factory

#### Webhook Type Guards

- **`isCalendarWebhook(payload)`** - Check if calendar webhook
- **`isCalendarSyncedWebhook(payload)`** - Check if calendar synced webhook
- **`isEventWebhook(payload)`** - Check if event webhook
- **`isReminderDueWebhook(payload)`** - Check if reminder due webhook
- **`isAttendeeWebhook(payload)`** - Check if attendee webhook
- **`isMemberWebhook(payload)`** - Check if member webhook

## Configuration

The `SchedulerClient` accepts the following configuration options:

```typescript
interface SchedulerConfig {
  baseURL: string;           // Required: Base URL of the go-scheduler API
  timeout?: number;          // Optional: Request timeout in milliseconds (default: 30000)
  headers?: Record<string, string>; // Optional: Custom headers to include in all requests
}
```

## Setting Custom Headers

You can set custom headers after initializing the client:

```typescript
// Set a header
client.setHeader("api-key", "new-token");

// Remove a header
client.removeHeader("api-key");
```

## Recurring Events

When working with recurring events, many operations support a `scope` parameter:

- `"single"`: Affects only the specific instance
- `"all"`: Affects all instances in the series
- `"future"`: Affects this instance and all future instances (for updates only)

```typescript
// Update only this instance
await client.events.update("instance-uid", {
  account_id: "user123",
  start_ts: newTime,
  end_ts: newTime + 3600,
  scope: "single"
});

// Update all instances
await client.events.update("master-uid", {
  account_id: "user123",
  start_ts: newTime,
  end_ts: newTime + 3600,
  scope: "all"
});
```

## Error Handling

The SDK throws errors with detailed information:

```typescript
try {
  const event = await client.events.get("invalid-uid");
} catch (error) {
  console.error("Error:", error.message);
  console.error("Status:", error.status);
  console.error("Response:", error.response);
}
```

## TypeScript Support

The SDK is written in TypeScript and includes full type definitions. Import types as needed:

```typescript
import {
  Calendar,
  Event,
  Reminder,
  Webhook,
  Attendee,
  CreateEventRequest,
  UpdateScope
} from "go-scheduler-node-sdk";
```

## API Reference

For complete API documentation, see the [go-scheduler API documentation](https://github.com/jaysongiroux/go-scheduler).

## License

MIT
