# Scheduler API Documentation

This document provides comprehensive API documentation for the Go Scheduler service.

> **Note:** For programmatic access using JavaScript/TypeScript, see the [Node.js SDK Documentation](./node-sdk.md).

## Table of Contents

- [Authentication](#authentication)
- [Response Format](#response-format)
- [Pagination](#pagination)
- [Timestamp Format](#timestamp-format)
- [Endpoints](#endpoints)
  - [Health Check](#health-check)
  - [Calendars](#calendars)
  - [Events](#events)
  - [Reminders](#reminders)
  - [Attendees](#attendees)
  - [Calendar Members](#calendar-members)
  - [ICS Import](#ics-import)
  - [Webhooks](#webhooks)
  - [Accounts](#accounts)

---

## Authentication

All API requests (except `/health`) require authentication using an API key.

**Header:**
```
api-key: your-api-key-here
```

**Status Codes:**
- `401 Unauthorized` - Missing or invalid API key

---

## Response Format

### Success Response

```json
{
  "data": { ... },
  "status": "success"
}
```

Or for resource creation/retrieval, the resource object is returned directly:

```json
{
  "calendar_uid": "uuid",
  "account_id": "account123",
  ...
}
```

### Error Response

```json
{
  "error": "Human-readable error message",
  "details": "Technical error details (optional)"
}
```

**Common Status Codes:**
- `200 OK` - Request successful
- `201 Created` - Resource created successfully
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication failed
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Pagination

Paginated endpoints accept the following query parameters:

- `limit` (integer, optional) - Number of results per page (default: 50, max: 1000)
- `offset` (integer, optional) - Number of results to skip (default: 0)

**Response Format:**
```json
{
  "data": [...],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 150
  }
}
```

---

## Timestamp Format

All timestamps are Unix timestamps (seconds since epoch) in UTC.

**Example:** `1705968143` represents `2024-01-22T20:49:03Z`

---

## Endpoints

---

## Health Check

### GET /health

Check the health status of the API.

**Authentication:** Not required

**Response:** `200 OK`
```json
{
  "status": "healthy",
  "time": "1705968143"
}
```

---

## Calendars

### POST /api/v1/calendars

Create a new calendar.

**Request Body:**
```json
{
  "calendar_uid": "uuid (optional, auto-generated)",
  "account_id": "string (required)",
  "settings": {},
  "metadata": {}
}
```

**Response:** `201 Created`
```json
{
  "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
  "account_id": "account123",
  "created_ts": 1705968143,
  "updated_ts": 1705968143,
  "settings": {},
  "metadata": {},
  "is_read_only": false,
  "sync_on_partial_failure": false
}
```

**Webhooks Triggered:**
- `calendar.created`

---

### GET /api/v1/calendars/{calendar_uid}

Get a calendar by UID with its members.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Response:** `200 OK`
```json
{
  "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
  "account_id": "account123",
  "created_ts": 1705968143,
  "updated_ts": 1705968143,
  "settings": {},
  "metadata": {},
  "is_read_only": false,
  "ics_url": null,
  "ics_auth_type": null,
  "ics_last_sync_ts": null,
  "ics_last_sync_status": null,
  "ics_sync_interval_seconds": null,
  "members": [
    {
      "account_id": "user456",
      "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
      "status": "confirmed",
      "role": "write",
      "invited_by": "account123",
      "invited_at_ts": 1705968100,
      "updated_ts": 1705968100
    }
  ]
}
```

---

### GET /api/v1/calendars

List calendars for a user (both owned and member calendars).

**Query Parameters:**
- `account_id` (string, required) - Account ID
- `limit` (integer, optional) - Page size
- `offset` (integer, optional) - Pagination offset

**Response:** `200 OK`
```json
{
  "data": [
    {
      "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
      "account_id": "account123",
      "is_owner": true,
      "created_ts": 1705968143,
      "updated_ts": 1705968143,
      "settings": {},
      "metadata": {},
      "members": [...]
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 5
  }
}
```

---

### PUT /api/v1/calendars/{calendar_uid}

Update a calendar.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Request Body:** (all fields optional except `calendar_uid` and `account_id`)
```json
{
  "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
  "account_id": "account123",
  "settings": { "timezone": "America/New_York" },
  "metadata": { "name": "Work Calendar", "color": "#FF5733" }
}
```

**Response:** `200 OK`
```json
{
  "calendar_uid": "550e8400-e29b-41d4-a716-446655440000",
  "account_id": "account123",
  "created_ts": 1705968143,
  "updated_ts": 1705968200,
  "settings": { "timezone": "America/New_York" },
  "metadata": { "name": "Work Calendar", "color": "#FF5733" }
}
```

**Webhooks Triggered:**
- `calendar.updated`

---

### DELETE /api/v1/calendars/{calendar_uid}

Delete a calendar and all its events.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Query Parameters:**
- `account_id` (string, required) - Account ID (must be calendar owner)

**Response:** `200 OK`
```json
{
  "status": "success"
}
```

**Webhooks Triggered:**
- `calendar.deleted`

---

### POST /api/v1/calendars/events

Query events across multiple calendars within a time range.

**Request Body:**
```json
{
  "calendar_uids": ["uuid1", "uuid2"],
  "start_ts": 1705968143,
  "end_ts": 1706054543
}
```

**Response:** `200 OK`
```json
{
  "events": [
    {
      "event_uid": "event-uuid",
      "calendar_uid": "cal-uuid",
      "account_id": "account123",
      "start_ts": 1705970000,
      "end_ts": 1705973600,
      "duration": 3600,
      "metadata": { "title": "Team Meeting" },
      "is_cancelled": false,
      "is_recurring_instance": false
    }
  ]
}
```

---

## Events

### POST /api/v1/events

Create a new event (single or recurring).

**Request Body:**
```json
{
  "event_uid": "uuid (optional)",
  "calendar_uid": "uuid (required)",
  "account_id": "string (required)",
  "start_ts": 1705970000,
  "end_ts": 1705973600,
  "duration": 3600,
  "metadata": {
    "title": "Team Meeting",
    "description": "Weekly sync",
    "location": "Conference Room A"
  },
  "timezone": "America/New_York (optional)",
  "local_start": "2024-01-22T15:00:00 (optional)",
  "recurrence": {
    "rule": "FREQ=WEEKLY;BYDAY=MO,WE,FR",
    "exceptions": []
  },
  "reminders": [
    {
      "account_id": "account123",
      "offset_seconds": -900,
      "metadata": { "type": "email" }
    }
  ],
  "attendees": [
    {
      "account_id": "user456",
      "role": "attendee",
      "metadata": { "email": "user@example.com" }
    }
  ]
}
```

**Response:** `201 Created`
```json
{
  "event_uid": "event-uuid",
  "calendar_uid": "cal-uuid",
  "account_id": "account123",
  "start_ts": 1705970000,
  "end_ts": 1705973600,
  "duration": 3600,
  "created_ts": 1705968143,
  "updated_ts": 1705968143,
  "metadata": { "title": "Team Meeting" },
  "recurrence": {
    "rule": "FREQ=WEEKLY;BYDAY=MO,WE,FR",
    "exceptions": []
  },
  "recurrence_status": "active",
  "is_recurring_instance": false,
  "is_modified": false,
  "is_cancelled": false,
  "exdates_ts": []
}
```

**Recurring Events:**
- For recurring events, specify the `recurrence` object with an RFC 5545 RRULE
- The API automatically generates instances based on the configured generation window
- `instances_generated` in the response indicates how many instances were created

**Timezone Support:**
- If `timezone` is provided, the event is DST-aware
- `local_start` can be used with `timezone` to create events in local time
- If `local_start` is provided, `start_ts` is computed from it

**Maximum Duration:** 24 hours (86400 seconds)

**Webhooks Triggered:**
- `event.created` (single or batch for series)

---

### GET /api/v1/events/{event_uid}

Get an event by UID.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Query Parameters:**
- `include_archived_reminders` (boolean, optional) - Include archived reminders (default: false)

**Response:** `200 OK`
```json
{
  "event_uid": "event-uuid",
  "calendar_uid": "cal-uuid",
  "account_id": "account123",
  "start_ts": 1705970000,
  "end_ts": 1705973600,
  "duration": 3600,
  "created_ts": 1705968143,
  "updated_ts": 1705968143,
  "metadata": { "title": "Team Meeting" },
  "is_cancelled": false,
  "is_recurring_instance": false,
  "reminders": [
    {
      "reminder_uid": "rem-uuid",
      "account_id": "account123",
      "offset_seconds": -900,
      "metadata": {}
    }
  ]
}
```

---

### PUT /api/v1/events/{event_uid}

Update an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Query Parameters:**
- `scope` (string, optional) - For recurring events: `single`, `future`, or `all` (default: `single`)

**Request Body:**
```json
{
  "event_uid": "event-uuid",
  "calendar_uid": "cal-uuid",
  "account_id": "account123",
  "start_ts": 1705971000,
  "end_ts": 1705974600,
  "metadata": { "title": "Updated Meeting" }
}
```

**Scope Behavior:**
- `single` - Update only this instance (creates a modified instance for recurring events)
- `future` - Update this and all future instances (creates a new master event)
- `all` - Update entire series (modifies the master event)

**Response:** `200 OK`
```json
{
  "event": { ... },
  "scope": "single",
  "affected_count": 1
}
```

**Webhooks Triggered:**
- `event.updated` (single or batch)

---

### DELETE /api/v1/events/{event_uid}

Delete an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Query Parameters:**
- `scope` (string, optional) - For recurring events: `single`, `future`, or `all` (default: `single`)

**Scope Behavior:**
- `single` - Delete only this instance (adds to exdates for recurring events)
- `future` - Delete this and all future instances (sets recurrence_end_ts)
- `all` - Delete entire series

**Response:** `200 OK`
```json
{
  "status": "success",
  "scope": "single",
  "deleted_count": 1
}
```

**Webhooks Triggered:**
- `event.deleted` (single or batch)

---

### POST /api/v1/events/{event_uid}/toggle-cancelled

Toggle the cancelled status of an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Response:** `200 OK`
```json
{
  "event_uid": "event-uuid",
  "is_cancelled": true,
  "updated_ts": 1705968200
}
```

**Webhooks Triggered:**
- `event.cancelled` or `event.uncancelled`

---

### POST /api/v1/events/{event_uid}/transfer-ownership

Transfer event ownership to another attendee.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Request Body:**
```json
{
  "new_organizer_account_id": "user456",
  "current_organizer_account_id": "account123"
}
```

**Response:** `200 OK`
```json
{
  "status": "success",
  "new_organizer": "user456",
  "old_organizer": "account123"
}
```

**Webhooks Triggered:**
- `event.ownership_transferred`

---

## Reminders

Reminders are always associated with a specific event and account.

### POST /api/v1/events/{event_uid}/reminders

Create a reminder for an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Request Body:**
```json
{
  "account_id": "account123",
  "offset_seconds": -900,
  "metadata": { "type": "email", "email": "user@example.com" },
  "scope": "single"
}
```

**Fields:**
- `offset_seconds` (integer, required) - Must be negative (time before event). E.g., `-900` = 15 minutes before
- `scope` (string, optional) - For recurring events: `single` or `all`

**Response:** `201 Created`
```json
{
  "reminder": {
    "reminder_uid": "rem-uuid",
    "event_uid": "event-uuid",
    "account_id": "account123",
    "offset_seconds": -900,
    "metadata": { "type": "email" },
    "created_ts": 1705968143,
    "is_delivered": false
  },
  "scope": "single",
  "count": 1
}
```

**For Series Reminders (`scope: all`):**
```json
{
  "reminder_group_id": "group-uuid",
  "master_event_uid": "master-uuid",
  "scope": "all",
  "count": 12
}
```

**Webhooks Triggered:**
- `reminder.created` (single or batch)
- `reminder.due` (when reminder should fire)

---

### GET /api/v1/events/{event_uid}/reminders

Get all reminders for an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Query Parameters:**
- `account_id` (string, optional) - Filter by account ID

**Response:** `200 OK`
```json
{
  "reminders": [
    {
      "reminder_uid": "rem-uuid",
      "event_uid": "event-uuid",
      "account_id": "account123",
      "offset_seconds": -900,
      "metadata": {},
      "is_delivered": false,
      "created_ts": 1705968143
    }
  ]
}
```

---

### PUT /api/v1/events/{event_uid}/reminders/{reminder_uid}

Update a reminder.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `reminder_uid` (uuid, required) - Reminder UID

**Request Body:**
```json
{
  "offset_seconds": -1800,
  "metadata": { "type": "push" },
  "scope": "single"
}
```

**Response:** `200 OK`
```json
{
  "reminder": { ... },
  "scope": "single",
  "count": 1
}
```

**Webhooks Triggered:**
- `reminder.updated` (single or batch)

---

### DELETE /api/v1/events/{event_uid}/reminders/{reminder_uid}

Delete a reminder.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `reminder_uid` (uuid, required) - Reminder UID

**Request Body:**
```json
{
  "scope": "single"
}
```

**Response:** `200 OK`
```json
{
  "status": "success",
  "scope": "single",
  "count": 1
}
```

**Webhooks Triggered:**
- `reminder.deleted` (single or batch)

---

## Attendees

Attendees represent participants in an event.

### POST /api/v1/events/{event_uid}/attendees

Add an attendee to an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Request Body:**
```json
{
  "account_id": "user456",
  "role": "attendee",
  "scope": "single",
  "metadata": {
    "email": "user@example.com",
    "name": "John Doe"
  }
}
```

**Fields:**
- `role` (string, required) - `attendee` or `organizer`
- `scope` (string, optional) - For recurring events: `single` or `all`

**Response:** `201 Created`
```json
{
  "attendee": {
    "event_uid": "event-uuid",
    "account_id": "user456",
    "role": "attendee",
    "rsvp_status": "pending",
    "metadata": { "email": "user@example.com" },
    "created_ts": 1705968143,
    "archived": false
  },
  "scope": "single",
  "count": 1
}
```

**RSVP Status Values:**
- `pending` - No response yet
- `accepted` - Attendee accepted
- `declined` - Attendee declined
- `tentative` - Attendee marked as tentative

**Webhooks Triggered:**
- `attendee.created` (single or batch)

---

### GET /api/v1/events/{event_uid}/attendees

Get all attendees for an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID

**Response:** `200 OK`
```json
{
  "attendees": [
    {
      "event_uid": "event-uuid",
      "account_id": "user456",
      "role": "attendee",
      "rsvp_status": "accepted",
      "metadata": {},
      "created_ts": 1705968143
    }
  ]
}
```

---

### GET /api/v1/events/{event_uid}/attendees/{account_id}

Get a specific attendee.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `account_id` (string, required) - Account ID

**Response:** `200 OK`
```json
{
  "event_uid": "event-uuid",
  "account_id": "user456",
  "role": "attendee",
  "rsvp_status": "accepted",
  "metadata": {},
  "created_ts": 1705968143
}
```

---

### GET /api/v1/attendees/events

Get all events where the account is an attendee.

**Query Parameters:**
- `account_id` (string, required) - Account ID
- `start_ts` (integer, required) - Start timestamp
- `end_ts` (integer, required) - End timestamp
- `rsvp_status` (string, optional) - Filter by RSVP status
- `limit` (integer, optional) - Page size
- `offset` (integer, optional) - Pagination offset

**Response:** `200 OK`
```json
{
  "data": [
    {
      "attendee": {
        "event_uid": "event-uuid",
        "account_id": "account123",
        "role": "attendee",
        "rsvp_status": "accepted"
      },
      "event": {
        "event_uid": "event-uuid",
        "start_ts": 1705970000,
        "metadata": { "title": "Meeting" }
      }
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 15
  }
}
```

---

### PATCH /api/v1/events/{event_uid}/attendees/{account_id}

Update attendee metadata.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `account_id` (string, required) - Account ID

**Request Body:**
```json
{
  "metadata": { "dietary_restrictions": "vegetarian" },
  "scope": "single"
}
```

**Response:** `200 OK`
```json
{
  "attendee": { ... },
  "scope": "single",
  "count": 1
}
```

**Webhooks Triggered:**
- `attendee.updated` (single or batch)

---

### PUT /api/v1/events/{event_uid}/attendees/{account_id}/rsvp

Update attendee RSVP status.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `account_id` (string, required) - Account ID

**Request Body:**
```json
{
  "rsvp_status": "accepted",
  "scope": "single"
}
```

**Values:** `pending`, `accepted`, `declined`, `tentative`

**Response:** `200 OK`
```json
{
  "attendee": { ... },
  "scope": "single",
  "count": 1
}
```

**Webhooks Triggered:**
- `attendee.rsvp_updated` (single or batch)

---

### DELETE /api/v1/events/{event_uid}/attendees/{account_id}

Remove an attendee from an event.

**Path Parameters:**
- `event_uid` (uuid, required) - Event UID
- `account_id` (string, required) - Account ID

**Request Body:**
```json
{
  "scope": "single"
}
```

**Response:** `200 OK`
```json
{
  "status": "success",
  "scope": "single",
  "count": 1
}
```

**Webhooks Triggered:**
- `attendee.deleted` (single or batch)

---

## Calendar Members

Calendar members allow sharing calendars with other users.

### POST /api/v1/calendars/{calendar_uid}/members

Invite members to a calendar.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Request Body:**
```json
{
  "account_id": "account123",
  "account_ids": ["user456", "user789"],
  "role": "write"
}
```

**Fields:**
- `account_id` (string, required) - Inviter's account ID (must be calendar owner)
- `account_ids` (array, required) - List of account IDs to invite
- `role` (string, optional) - `read` or `write` (default: `write`)

**Response:** `201 Created`
```json
{
  "invited_count": 2,
  "members": [
    {
      "account_id": "user456",
      "calendar_uid": "cal-uuid",
      "status": "pending",
      "role": "write",
      "invited_by": "account123",
      "invited_at_ts": 1705968143,
      "updated_ts": 1705968143
    }
  ]
}
```

**Webhooks Triggered:**
- `member.invited` (one per member)

---

### GET /api/v1/calendars/{calendar_uid}/members

Get all members of a calendar.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Query Parameters:**
- `account_id` (string, required) - Requesting account ID

**Response:** `200 OK`
```json
{
  "members": [
    {
      "account_id": "user456",
      "calendar_uid": "cal-uuid",
      "status": "confirmed",
      "role": "write",
      "invited_by": "account123",
      "invited_at_ts": 1705968100,
      "updated_ts": 1705968143
    }
  ]
}
```

---

### PUT /api/v1/calendars/{calendar_uid}/members/{account_id}

Update a calendar member (status or role).

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID
- `account_id` (string, required) - Member account ID

**Request Body:**
```json
{
  "status": "confirmed",
  "role": "read"
}
```

**Fields (all optional):**
- `status` (string) - `pending` or `confirmed`
- `role` (string) - `read` or `write`

**Response:** `200 OK`
```json
{
  "account_id": "user456",
  "calendar_uid": "cal-uuid",
  "status": "confirmed",
  "role": "read",
  "updated_ts": 1705968200
}
```

**Webhooks Triggered:**
- `member.status_updated` or `member.role_updated`

---

### DELETE /api/v1/calendars/{calendar_uid}/members/{account_id}

Remove a member from a calendar.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID
- `account_id` (string, required) - Member account ID

**Response:** `200 OK`
```json
{
  "status": "success"
}
```

**Webhooks Triggered:**
- `member.removed`

---

## ICS Import

Import events from ICS (iCalendar) files or URLs.

### POST /api/v1/calendars/import/ics

Import events from an ICS file.

**Content-Type:** `multipart/form-data`

**Form Fields:**
- `file` (file, required) - ICS file to import
- `account_id` (string, required) - Account ID
- `calendar_uid` (uuid, optional) - Existing calendar UID to import into
- `calendar_metadata` (JSON string, optional) - Metadata for new calendar if not using existing
- `import_reminders` (boolean, optional) - Import reminders from ICS (default: true)
- `import_attendees` (boolean, optional) - Import attendees from ICS (default: true)

**Response:** `200 OK`
```json
{
  "calendar": {
    "calendar_uid": "cal-uuid",
    "account_id": "account123",
    "metadata": { "name": "Imported Calendar" }
  },
  "summary": {
    "total_events": 10,
    "imported_events": 9,
    "failed_events": 1
  },
  "events": [
    {
      "ics_uid": "event1@example.com",
      "event_uid": "event-uuid",
      "status": "success"
    },
    {
      "ics_uid": "event2@example.com",
      "status": "failed",
      "error": "Invalid date format"
    }
  ]
}
```

**Webhooks Triggered:**
- `calendar.created` (if new calendar)
- `event.created` (batch)

---

### POST /api/v1/calendars/import/ics-link

Import events from an ICS URL with automatic synchronization.

**Request Body:**
```json
{
  "account_id": "account123",
  "ics_url": "https://calendar.example.com/feed.ics",
  "auth_type": "none",
  "auth_credentials": "optional-for-basic-or-bearer",
  "sync_interval_seconds": 86400,
  "calendar_metadata": {
    "name": "External Calendar",
    "color": "#4285F4"
  },
  "sync_on_partial_failure": false
}
```

**Fields:**
- `ics_url` (string, required) - HTTP/HTTPS URL to ICS feed
- `auth_type` (string, required) - `none`, `basic`, or `bearer`
- `auth_credentials` (string, optional) - For `basic`: `username:password`, for `bearer`: `token`
- `sync_interval_seconds` (integer, optional) - Sync frequency (default: 86400 = 24 hours)
- `sync_on_partial_failure` (boolean, optional) - Continue sync even if some events fail (default: false)

**Response:** `200 OK`
```json
{
  "calendar": {
    "calendar_uid": "cal-uuid",
    "account_id": "account123",
    "is_read_only": true,
    "ics_url": "https://calendar.example.com/feed.ics",
    "ics_auth_type": "none",
    "ics_last_sync_ts": 1705968143,
    "ics_last_sync_status": "success",
    "ics_sync_interval_seconds": 86400,
    "metadata": { "name": "External Calendar" }
  },
  "summary": {
    "total_events": 15,
    "imported_events": 15,
    "failed_events": 0
  },
  "events": [...],
  "sync_scheduled": true
}
```

**Important Notes:**
- Creates a **read-only** calendar that cannot be merged with existing calendars
- Events in the calendar cannot be created, edited, or deleted manually
- The calendar is automatically synced based on `sync_interval_seconds`
- Credentials are encrypted at rest using AES-256-GCM
- Does not follow HTTP redirects for security

**Sync Status Values:**
- `success` - Last sync completed successfully
- `stale` - Last sync failed, data may be outdated
- `failed` - Sync is failing consistently

**Webhooks Triggered:**
- `calendar.created`
- `calendar.synced`
- `event.created` (batch)

---

### POST /api/v1/calendars/{calendar_uid}/resync

Manually trigger a resync for an ICS-linked calendar.

**Path Parameters:**
- `calendar_uid` (uuid, required) - Calendar UID

**Response:** `200 OK`
```json
{
  "calendar": {
    "calendar_uid": "cal-uuid",
    "ics_last_sync_ts": 1705968200,
    "ics_last_sync_status": "success"
  },
  "imported_events": 15,
  "failed_events": 0,
  "warnings": []
}
```

**Error Response:** `400 Bad Request`
```json
{
  "error": "Calendar is not an ICS-linked calendar"
}
```

**Webhooks Triggered:**
- `calendar.resynced`
- `event.created` (batch, if new events)
- `event.deleted` (batch, if events removed from feed)

---

## Webhooks

Configure webhooks to receive real-time notifications about events.

### POST /api/v1/webhooks

Create a webhook endpoint.

**Request Body:**
```json
{
  "webhook_uid": "uuid (optional)",
  "url": "https://your-app.com/webhooks",
  "event_types": [
    "event.created",
    "event.updated",
    "calendar.created"
  ],
  "is_active": true,
  "retry_count": 3,
  "timeout_seconds": 10,
  "secret": "optional-auto-generated"
}
```

**Event Types:**
- `calendar.created`, `calendar.updated`, `calendar.deleted`, `calendar.synced`, `calendar.resynced`
- `event.created`, `event.updated`, `event.deleted`, `event.cancelled`, `event.uncancelled`
- `event.ownership_transferred`
- `reminder.created`, `reminder.updated`, `reminder.deleted`, `reminder.due`
- `attendee.created`, `attendee.updated`, `attendee.deleted`, `attendee.rsvp_updated`
- `member.invited`, `member.status_updated`, `member.role_updated`, `member.removed`

**Response:** `201 Created`
```json
{
  "webhook_uid": "webhook-uuid",
  "url": "https://your-app.com/webhooks",
  "event_types": ["event.created"],
  "is_active": true,
  "retry_count": 3,
  "timeout_seconds": 10,
  "secret": "whsec_...",
  "created_ts": 1705968143,
  "updated_ts": 1705968143
}
```

**Webhook Payload Format:**
```json
{
  "webhook_uid": "webhook-uuid",
  "event_type": "event.created",
  "delivery_id": "delivery-uuid",
  "timestamp": 1705968143,
  "data": {
    "event_uid": "event-uuid",
    "calendar_uid": "cal-uuid",
    ...
  }
}
```

**Webhook Signature:**
Webhooks include an `X-Webhook-Signature` header containing an HMAC-SHA256 signature:
```
X-Webhook-Signature: sha256=<signature>
```

Compute the signature using the webhook secret:
```javascript
const crypto = require('crypto');
const signature = crypto
  .createHmac('sha256', webhookSecret)
  .update(JSON.stringify(payload))
  .digest('hex');
```

---

### GET /api/v1/webhooks/{webhook_uid}

Get a webhook by UID.

**Path Parameters:**
- `webhook_uid` (uuid, required) - Webhook UID

**Response:** `200 OK`
```json
{
  "webhook_uid": "webhook-uuid",
  "url": "https://your-app.com/webhooks",
  "event_types": ["event.created"],
  "is_active": true,
  "created_ts": 1705968143
}
```

---

### GET /api/v1/webhooks

List all webhook endpoints.

**Query Parameters:**
- `limit` (integer, optional) - Page size
- `offset` (integer, optional) - Pagination offset

**Response:** `200 OK`
```json
{
  "data": [
    {
      "webhook_uid": "webhook-uuid",
      "url": "https://your-app.com/webhooks",
      "event_types": ["event.created"],
      "is_active": true
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 3
  }
}
```

---

### PUT /api/v1/webhooks/{webhook_uid}

Update a webhook endpoint.

**Path Parameters:**
- `webhook_uid` (uuid, required) - Webhook UID

**Request Body:** (all fields optional except webhook_uid)
```json
{
  "url": "https://new-url.com/webhooks",
  "event_types": ["event.created", "event.updated"],
  "is_active": false,
  "retry_count": 5,
  "timeout_seconds": 15
}
```

**Note:** The `secret` field cannot be updated.

**Response:** `200 OK`
```json
{
  "webhook_uid": "webhook-uuid",
  "url": "https://new-url.com/webhooks",
  "event_types": ["event.created", "event.updated"],
  "is_active": false,
  "updated_ts": 1705968200
}
```

---

### DELETE /api/v1/webhooks/{webhook_uid}

Delete a webhook endpoint.

**Path Parameters:**
- `webhook_uid` (uuid, required) - Webhook UID

**Response:** `200 OK`
```json
{
  "status": "success"
}
```

---

### GET /api/v1/webhooks/deliveries/{webhook_uid}

Get delivery history for a webhook.

**Path Parameters:**
- `webhook_uid` (uuid, required) - Webhook UID

**Query Parameters:**
- `limit` (integer, optional) - Page size
- `offset` (integer, optional) - Pagination offset

**Response:** `200 OK`
```json
{
  "data": [
    {
      "delivery_id": "delivery-uuid",
      "webhook_uid": "webhook-uuid",
      "event_type": "event.created",
      "payload": { ... },
      "status": "success",
      "status_code": 200,
      "attempt_count": 1,
      "created_ts": 1705968143,
      "delivered_ts": 1705968144
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 120
  }
}
```

**Status Values:**
- `pending` - Queued for delivery
- `success` - Delivered successfully (2xx response)
- `failed` - Delivery failed after all retries

---

## Accounts

### DELETE /api/v1/accounts

Delete an account and all associated data.

**Query Parameters:**
- `account_id` (string, required) - Account ID to delete

**Response:** `200 OK`
```json
{
  "status": "success"
}
```

**Cascading Deletes:**
This endpoint deletes:
- All calendars owned by the account
- All events in those calendars
- All reminders for the account
- All attendee records
- All calendar memberships

**Warning:** This operation is irreversible.

---

## Error Codes

### Common Errors

#### 400 Bad Request
```json
{
  "error": "Invalid request body",
  "details": "json: cannot unmarshal string into Go value of type int64"
}
```

**Common Causes:**
- Missing required fields
- Invalid field types
- Invalid UUIDs
- Invalid date/time values
- Invalid RRULE syntax

#### 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "details": "Invalid or missing API key"
}
```

#### 403 Forbidden
```json
{
  "error": "Cannot create reminders on read-only calendar"
}
```

**Common Causes:**
- Attempting to modify a read-only ICS-linked calendar
- Insufficient permissions (not calendar owner/member)
- Invalid member role

#### 404 Not Found
```json
{
  "error": "Calendar not found",
  "details": "sql: no rows in result set"
}
```

#### 500 Internal Server Error
```json
{
  "error": "Failed to create event",
  "details": "database connection failed"
}
```

---

## Rate Limiting

Currently, the API does not enforce rate limiting. This may be added in a future version.

---

## Webhooks Reference

### Webhook Event Payloads

#### calendar.created
```json
{
  "event_type": "calendar.created",
  "data": {
    "calendar_uid": "uuid",
    "account_id": "account123",
    "created_ts": 1705968143
  }
}
```

#### calendar.synced
```json
{
  "event_type": "calendar.synced",
  "data": {
    "calendar_uid": "uuid",
    "account_id": "account123",
    "imported_events": 15,
    "failed_events": 0,
    "warnings": [],
    "sync_ts": 1705968143,
    "manual_trigger": false
  }
}
```

#### event.created (single)
```json
{
  "event_type": "event.created",
  "data": {
    "event_uid": "uuid",
    "calendar_uid": "uuid",
    "account_id": "account123",
    "start_ts": 1705970000
  }
}
```

#### event.created (batch)
```json
{
  "event_type": "event.created",
  "data": [
    {
      "event_uid": "uuid1",
      "calendar_uid": "uuid",
      "start_ts": 1705970000
    },
    {
      "event_uid": "uuid2",
      "calendar_uid": "uuid",
      "start_ts": 1706056400
    }
  ]
}
```

#### reminder.due
```json
{
  "event_type": "reminder.due",
  "data": {
    "reminder_uid": "uuid",
    "event_uid": "uuid",
    "account_id": "account123",
    "offset_seconds": -900,
    "remind_at_ts": 1705969100,
    "event": {
      "event_uid": "uuid",
      "start_ts": 1705970000,
      "metadata": { "title": "Meeting" }
    }
  }
}
```

---

## Best Practices

### 1. Use UUIDs for Idempotency
Generate UUIDs client-side for `calendar_uid`, `event_uid`, etc., to ensure idempotent creates.

### 2. Handle Recurring Events Carefully
- Always specify `scope` when updating/deleting recurring events
- Be aware that `scope=future` creates a new master event
- Use `exdates_ts` to check for cancelled instances

### 3. Timezone Support
- Use `timezone` and `local_start` for DST-aware events
- Store timestamps in UTC (Unix timestamp)
- Compute local time on the client for display

### 4. Webhook Best Practices
- Verify webhook signatures to ensure authenticity
- Return a 2xx status code quickly (< 5 seconds)
- Process webhook payloads asynchronously
- Handle duplicate deliveries (use `delivery_id` for deduplication)

### 5. Pagination
- Always use `limit` and `offset` for large result sets
- Default page size is 50, max is 1000
- Check `pagination.total` to determine if there are more results

### 6. ICS Import
- Use `sync_on_partial_failure: true` for unreliable feeds
- Monitor `ics_last_sync_status` and `ics_error_message` fields
- Respect the `is_read_only` flag on ICS-linked calendars
- Set appropriate `sync_interval_seconds` based on feed update frequency

### 7. Error Handling
- Check for `400` errors and validate input before sending
- Implement exponential backoff for `500` errors
- Log `details` field for debugging

---

## Changelog

### Version 1.0 (2024-01-22)
- Initial API release
- Calendar, event, reminder, attendee, and webhook management
- Recurring events with RRULE support
- ICS file and URL import with automatic synchronization
- Timezone and DST support
- Read-only ICS-linked calendars
- Webhook delivery with signatures and retries

---

## Support

For API support, bug reports, or feature requests, please contact your system administrator or refer to the project repository.

For SDK documentation, see [Node.js SDK Documentation](./node-sdk.md).
