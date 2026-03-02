import { describe, expect, test, beforeAll, afterAll } from "vitest";
import { startICSServer } from "../helpers/ics-server";
import {
  CalendarObject,
  ErrorObject,
  EventObject,
  GenericPagedResponse,
  ICSImportResponse,
  ICSLinkImportResponse,
  ResyncResponse,
} from "../helpers/types";
import {
  createCalendar,
  deleteCalendar,
  importICSLink,
  resyncCalendar,
  getCalendar,
} from "../helpers/calendar";
import { getCalendarEvents } from "../helpers/event";

// Simple ICS content for testing
const simpleICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-123
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Test Meeting
DESCRIPTION:A test meeting for import
LOCATION:Conference Room A
END:VEVENT
END:VCALENDAR`;

// ICS with multiple events
const multiEventICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-1
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Event One
END:VEVENT
BEGIN:VEVENT
UID:event-2
DTSTART:20260116T140000Z
DTEND:20260116T150000Z
SUMMARY:Event Two
END:VEVENT
BEGIN:VEVENT
UID:event-3
DTSTART:20260117T090000Z
DTEND:20260117T100000Z
SUMMARY:Event Three
END:VEVENT
END:VCALENDAR`;

// Updated ICS with one event modified
const updatedICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-1
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Event One UPDATED
END:VEVENT
BEGIN:VEVENT
UID:event-2
DTSTART:20260116T140000Z
DTEND:20260116T150000Z
SUMMARY:Event Two
END:VEVENT
BEGIN:VEVENT
UID:event-4
DTSTART:20260118T130000Z
DTEND:20260118T140000Z
SUMMARY:Event Four NEW
END:VEVENT
END:VCALENDAR`;

describe("ICS URL Import API", () => {
  const accountId = crypto.randomUUID();
  const createdCalendarUids: string[] = [];
  const BASE_PORT = Math.floor(Math.random() * 1000) + 9000;
  let portCounter = 0;

  // Helper to get next available port
  const getNextPort = () => BASE_PORT + portCounter++;

  // Helper to clean up calendars after tests
  const cleanup = async () => {
    for (const uid of createdCalendarUids) {
      try {
        await deleteCalendar({ calendarUid: uid });
      } catch {
        // Ignore cleanup errors
      }
    }
  };

  test("Should import ICS from URL without authentication", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
        syncIntervalSeconds: 3600,
        calendarMetadata: { name: "Imported from URL", source: "ics_url" },
      })) as ICSLinkImportResponse;

      expect(response.calendar).toBeDefined();
      expect(response.calendar.calendar_uid).toBeDefined();
      expect(response.calendar.account_id).toBe(accountId);
      expect(response.calendar.is_read_only).toBe(true);
      expect(response.calendar.ics_url).toBe(
        `http://localhost:${port}/calendar.ics`,
      );
      expect(response.calendar.ics_sync_interval_seconds).toBe(3600);

      expect(response.summary.total_events).toBe(1);
      expect(response.summary.imported_events).toBe(1);
      expect(response.summary.failed_events).toBe(0);
      expect(response.sync_scheduled).toBe(true);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should import ICS from URL with Basic Auth", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent, {
      requireAuth: true,
      username: "testuser",
      password: "testpass",
    });

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "basic",
        authCredentials: "testuser:testpass",
        syncIntervalSeconds: 86400,
      })) as ICSLinkImportResponse;

      expect(response.calendar).toBeDefined();
      expect(response.calendar.is_read_only).toBe(true);
      expect(response.calendar.ics_auth_type).toBe("basic");
      expect(response.summary.imported_events).toBe(1);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should import ICS from URL with Bearer Token", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent, {
      requireAuth: true,
      bearerToken: "secret-token-12345",
    });

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "bearer",
        authCredentials: "secret-token-12345",
      })) as ICSLinkImportResponse;

      expect(response.calendar).toBeDefined();
      expect(response.calendar.is_read_only).toBe(true);
      expect(response.calendar.ics_auth_type).toBe("bearer");
      expect(response.summary.imported_events).toBe(1);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should fail with wrong Basic Auth credentials", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent, {
      requireAuth: true,
      username: "testuser",
      password: "testpass",
    });

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "basic",
        authCredentials: "wronguser:wrongpass",
      })) as ErrorObject;

      expect(response.error).toBeDefined();
      expect(response.error).toContain("Failed to fetch ICS");
    } finally {
      await server.close();
    }
  });

  test("Should import multiple events from URL", async () => {
    const port = getNextPort();
    const server = startICSServer(port, multiEventICSContent);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
      })) as ICSLinkImportResponse;

      expect(response.summary.total_events).toBe(3);
      expect(response.summary.imported_events).toBe(3);
      expect(response.summary.failed_events).toBe(0);
      expect(response.events).toHaveLength(3);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should handle custom sync interval", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
        syncIntervalSeconds: 7200, // 2 hours
      })) as ICSLinkImportResponse;

      expect(response.calendar.ics_sync_interval_seconds).toBe(7200);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should fail with invalid URL", async () => {
    const response = (await importICSLink({
      accountId: accountId,
      icsUrl: "not-a-valid-url",
      authType: "none",
    })) as ErrorObject;

    expect(response.error).toBeDefined();
  });

  test("Should fail with missing account_id", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      const headers: Record<string, string> = {
        "api-key": process.env.SCHEDULER_API_KEY || "",
      };

      const httpResponse = await fetch(
        `${process.env.SCHEDULER_URL}/api/v1/calendars/import/ics-link`,
        {
          method: "POST",
          headers: {
            ...headers,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            ics_url: `http://localhost:${port}/calendar.ics`,
            auth_type: "none",
          }),
        },
      );

      const response = (await httpResponse.json()) as ErrorObject;
      expect(response.error).toBe("account_id is required");
    } finally {
      await server.close();
    }
  });

  test("Should fail with invalid auth_type", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "invalid" as any,
      })) as ErrorObject;

      expect(response.error).toContain("auth_type");
    } finally {
      await server.close();
    }
  });

  test("Should fail with sync interval too short", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
        syncIntervalSeconds: 60, // Too short (minimum is 300)
      })) as ErrorObject;

      expect(response.error).toContain("sync_interval_seconds");
    } finally {
      await server.close();
    }
  });

  test("Should manually resync an imported calendar", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      // First import
      const importResponse = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
      })) as ICSLinkImportResponse;

      expect(importResponse.summary.imported_events).toBe(1);
      createdCalendarUids.push(importResponse.calendar.calendar_uid);

      // Update ICS content on server
      server.updateContent(multiEventICSContent);

      // Manually resync
      const resyncResponse = (await resyncCalendar({
        calendarUid: importResponse.calendar.calendar_uid,
      })) as ResyncResponse;

      expect(resyncResponse.imported_events).toBe(3);
      expect(resyncResponse.failed_events).toBe(0);
      expect(resyncResponse.calendar.ics_last_sync_status).toBe("success");

      // Verify events were replaced
      const eventsResponse = (await getCalendarEvents({
        calendarUids: [importResponse.calendar.calendar_uid],
        startTs: 1768467600, // 2026-01-15 00:00:00 UTC
        endTs: 1770282000, // 2026-02-05 00:00:00 UTC
      })) as GenericPagedResponse<EventObject>;

      expect(eventsResponse.data.length).toBe(3);
    } finally {
      await server.close();
    }
  });

  test("Should handle resync with If-Modified-Since", async () => {
    const port = getNextPort();
    const lastModified = new Date().toUTCString();
    const server = startICSServer(port, simpleICSContent, {
      lastModified: lastModified,
    });

    try {
      // First import
      const importResponse = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
      })) as ICSLinkImportResponse;

      createdCalendarUids.push(importResponse.calendar.calendar_uid);

      // Check that last_modified was stored
      const calendar = (await getCalendar({
        calendarUid: importResponse.calendar.calendar_uid,
      })) as CalendarObject;

      expect(calendar.ics_last_modified).toBe(lastModified);
    } finally {
      await server.close();
    }
  });

  test("Should create read-only calendar that prevents event creation", async () => {
    const port = getNextPort();
    const server = startICSServer(port, simpleICSContent);

    try {
      // Import calendar
      const importResponse = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
      })) as ICSLinkImportResponse;

      createdCalendarUids.push(importResponse.calendar.calendar_uid);

      // Try to create an event on the read-only calendar
      const headers: Record<string, string> = {
        "api-key": process.env.SCHEDULER_API_KEY || "",
      };

      const createEventResponse = await fetch(
        `${process.env.SCHEDULER_URL}/api/v1/events`,
        {
          method: "POST",
          headers: {
            ...headers,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            calendar_uid: importResponse.calendar.calendar_uid,
            account_id: accountId,
            start_ts: Math.floor(Date.now() / 1000) + 3600,
            end_ts: Math.floor(Date.now() / 1000) + 7200,
            metadata: { title: "New Event" },
          }),
        },
      );

      const errorResponse = (await createEventResponse.json()) as ErrorObject;
      expect(errorResponse.error).toContain("read-only");
    } finally {
      await server.close();
    }
  });

  test("Should handle partial failure mode", async () => {
    const port = getNextPort();
    // ICS with one valid and one invalid event (missing required fields)
    const partialICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:valid-event
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Valid Event
END:VEVENT
BEGIN:VEVENT
UID:invalid-event
SUMMARY:Invalid Event Without Dates
END:VEVENT
END:VCALENDAR`;

    const server = startICSServer(port, partialICS);

    try {
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
        syncOnPartialFailure: true,
      })) as ICSLinkImportResponse;

      // Should have imported the valid event despite the invalid one
      expect(response.summary.total_events).toBe(2);
      expect(response.summary.imported_events).toBeGreaterThan(0);

      createdCalendarUids.push(response.calendar.calendar_uid);
    } finally {
      await server.close();
    }
  });

  test("Should fail resync on non-ICS calendar", async () => {
    // Create a regular calendar
    const regularCal = (await createCalendar({
      accountId: accountId,
      metadata: { name: "Regular Calendar" },
    })) as CalendarObject;

    createdCalendarUids.push(regularCal.calendar_uid);

    // Try to resync
    const response = (await resyncCalendar({
      calendarUid: regularCal.calendar_uid,
    })) as ErrorObject;

    expect(response.error).toContain("not an ICS import");
  });

  test("Should store ETag from ICS server", async () => {
    const port = getNextPort();
    const etag = '"your-secret-api-key-here"';
    const server = startICSServer(port, simpleICSContent, {
      etag: etag,
    });

    try {
      const importResponse = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${port}/calendar.ics`,
        authType: "none",
      })) as ICSLinkImportResponse;

      createdCalendarUids.push(importResponse.calendar.calendar_uid);

      // Check that etag was stored
      const calendar = (await getCalendar({
        calendarUid: importResponse.calendar.calendar_uid,
      })) as CalendarObject;

      expect(calendar.ics_last_etag).toBe(etag);
    } finally {
      await server.close();
    }
  });

  // Cleanup after all tests
  test.afterAll(async () => {
    await cleanup();
  });
});
