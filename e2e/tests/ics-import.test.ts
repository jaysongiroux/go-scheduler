import { describe, expect, test } from "vitest";
import * as fs from "fs";
import * as path from "path";
import {
  CalendarObject,
  ErrorObject,
  EventObject,
  GenericPagedResponse,
  ICSImportResponse,
} from "../helpers/types";
import { createCalendar, deleteCalendar, importICS } from "../helpers/calendar";
import { getCalendarEvents } from "../helpers/event";

// Simple ICS content for basic testing
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

// ICS with recurring event
const recurringICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:recurring-event-123
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Weekly Team Meeting
RRULE:FREQ=WEEKLY;COUNT=4
END:VEVENT
END:VCALENDAR`;

// ICS with reminder
const reminderICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-with-reminder
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Event With Reminder
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
DESCRIPTION:Reminder: Event starting in 15 minutes
END:VALARM
END:VEVENT
END:VCALENDAR`;

describe("ICS Import API", () => {
  const accountId = crypto.randomUUID();
  const createdCalendarUids: string[] = [];

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

  test("Should import ICS and create a new calendar", async () => {
    const response = (await importICS({
      file: Buffer.from(simpleICSContent),
      accountId: accountId,
      calendarMetadata: { name: "Imported Calendar", source: "ics" },
    })) as ICSImportResponse;

    expect(response.calendar).toBeDefined();
    expect(response.calendar.calendar_uid).toBeDefined();
    expect(response.calendar.account_id).toBe(accountId);
    expect(response.calendar.metadata).toEqual({
      name: "Imported Calendar",
      source: "ics",
    });

    expect(response.summary.total_events).toBe(1);
    expect(response.summary.imported_events).toBe(1);
    expect(response.summary.failed_events).toBe(0);

    expect(response.events).toHaveLength(1);
    expect(response.events[0].ics_uid).toBe("test-event-123");
    expect(response.events[0].status).toBe("success");
    expect(response.events[0].event_uid).toBeDefined();

    createdCalendarUids.push(response.calendar.calendar_uid);
  });

  test("Should import ICS into an existing calendar", async () => {
    // Create a calendar first
    const calResponse = (await createCalendar({
      accountId: accountId,
      metadata: { name: "Existing Calendar" },
    })) as CalendarObject;
    createdCalendarUids.push(calResponse.calendar_uid);

    // Import into existing calendar
    const response = (await importICS({
      file: Buffer.from(simpleICSContent),
      accountId: accountId,
      calendarUid: calResponse.calendar_uid,
    })) as ICSImportResponse;

    expect(response.calendar.calendar_uid).toBe(calResponse.calendar_uid);
    expect(response.summary.imported_events).toBe(1);
  });

  test("Should import multiple events from ICS", async () => {
    const response = (await importICS({
      file: Buffer.from(multiEventICSContent),
      accountId: accountId,
    })) as ICSImportResponse;

    expect(response.summary.total_events).toBe(3);
    expect(response.summary.imported_events).toBe(3);
    expect(response.summary.failed_events).toBe(0);
    expect(response.events).toHaveLength(3);

    // Verify all events were created successfully
    for (const eventResult of response.events) {
      expect(eventResult.status).toBe("success");
      expect(eventResult.event_uid).toBeDefined();
    }

    createdCalendarUids.push(response.calendar.calendar_uid);
  });

  test("Should import recurring event from ICS", async () => {
    const response = (await importICS({
      file: Buffer.from(recurringICSContent),
      accountId: accountId,
    })) as ICSImportResponse;

    expect(response.summary.total_events).toBe(1);
    expect(response.summary.imported_events).toBe(1);
    expect(response.events[0].status).toBe("success");

    createdCalendarUids.push(response.calendar.calendar_uid);

    // Verify events were created (master + instances)
    const eventsResponse = (await getCalendarEvents({
      calendarUids: [response.calendar.calendar_uid],
      startTs: 1768467600, // 2026-01-15 00:00:00 UTC
      endTs: 1770282000, // 2026-02-05 00:00:00 UTC
    })) as GenericPagedResponse<EventObject>;

    // Should have master event + instances
    expect(eventsResponse.data).toBeDefined();
    expect(eventsResponse.data.length).toBeGreaterThan(1);
  });

  test("Should import event with reminder", async () => {
    const response = (await importICS({
      file: Buffer.from(reminderICSContent),
      accountId: accountId,
      importReminders: true,
    })) as ICSImportResponse;

    expect(response.summary.imported_events).toBe(1);
    expect(response.events[0].status).toBe("success");

    createdCalendarUids.push(response.calendar.calendar_uid);
  });

  test("Should skip reminders when importReminders is false", async () => {
    const response = (await importICS({
      file: Buffer.from(reminderICSContent),
      accountId: accountId,
      importReminders: false,
    })) as ICSImportResponse;

    expect(response.summary.imported_events).toBe(1);

    createdCalendarUids.push(response.calendar.calendar_uid);
  });

  test("Should fail without account_id", async () => {
    const headers: Record<string, string> = {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    };

    const formData = new FormData();
    formData.append("file", new Blob([simpleICSContent]), "calendar.ics");
    // Intentionally not adding account_id

    const httpResponse = await fetch(
      `${process.env.SCHEDULER_URL}/api/v1/calendars/import/ics`,
      {
        method: "POST",
        headers,
        body: formData,
      }
    );

    const response = (await httpResponse.json()) as ErrorObject;
    expect(response.error).toBe("account_id is required");
  });

  test("Should fail without file", async () => {
    const headers: Record<string, string> = {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    };

    const formData = new FormData();
    formData.append("account_id", accountId);
    // Intentionally not adding file

    const httpResponse = await fetch(
      `${process.env.SCHEDULER_URL}/api/v1/calendars/import/ics`,
      {
        method: "POST",
        headers,
        body: formData,
      }
    );

    const response = (await httpResponse.json()) as ErrorObject;
    expect(response.error).toBe("file is required");
  });

  test("Should fail with invalid calendar_uid", async () => {
    const response = (await importICS({
      file: Buffer.from(simpleICSContent),
      accountId: accountId,
      calendarUid: "invalid-uuid",
    })) as ErrorObject;

    expect(response.error).toBe("Invalid calendar_uid");
  });

  test("Should fail with non-existent calendar_uid", async () => {
    const response = (await importICS({
      file: Buffer.from(simpleICSContent),
      accountId: accountId,
      calendarUid: crypto.randomUUID(),
    })) as ErrorObject;

    expect(response.error).toBe("Calendar not found");
  });

  test("Should import Formula 2026 calendar", async () => {
    // Read the Formula 2026 ICS file
    const icsFilePath = path.join(
      __dirname,
      "../files/calendar-formula-2026.ics"
    );
    const icsContent = fs.readFileSync(icsFilePath);

    const response = (await importICS({
      file: icsContent,
      accountId: accountId,
      calendarMetadata: { name: "Formula 1 2026 Season", sport: "motorsport" },
    })) as ICSImportResponse;

    expect(response.calendar).toBeDefined();
    expect(response.calendar.metadata).toEqual({
      name: "Formula 1 2026 Season",
      sport: "motorsport",
    });

    // Should have imported events
    expect(response.summary.total_events).toBeGreaterThan(0);
    expect(response.summary.imported_events).toBeGreaterThan(0);

    // All events should have succeeded or the total should match
    expect(
      response.summary.imported_events + response.summary.failed_events
    ).toBe(response.summary.total_events);

    createdCalendarUids.push(response.calendar.calendar_uid);
  });

  // Cleanup after all tests
  test.afterAll(async () => {
    await cleanup();
  });
});
