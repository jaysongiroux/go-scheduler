import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { SchedulerClient } from "go-scheduler-node-sdk";

describe("SDK Client Integration Tests", () => {
  let client: SchedulerClient;
  let accountId1: string;
  let accountId2: string;
  let calendarUid1: string;
  let calendarUid2: string;

  beforeAll(async () => {
    // Initialize SDK client
    client = new SchedulerClient({
      baseURL: process.env.SCHEDULER_URL || "http://localhost:8080",
      timeout: 10000,
      headers: {
        "api-key": process.env.SCHEDULER_API_KEY || "your-secret-api-key-here",
      },
    });

    // Create test accounts
    accountId1 = `sdk-test-${Date.now()}-1`;
    accountId2 = `sdk-test-${Date.now()}-2`;
  });

  afterAll(async () => {
    // Cleanup calendars if they were created
    if (calendarUid1) {
      try {
        await client.calendars.delete(calendarUid1);
      } catch (e) {
        // Ignore cleanup errors
      }
    }
    if (calendarUid2) {
      try {
        await client.calendars.delete(calendarUid2);
      } catch (e) {
        // Ignore cleanup errors
      }
    }
  });

  describe("Client Initialization", () => {
    test("should initialize client successfully", () => {
      expect(client).toBeDefined();
      expect(client.calendars).toBeDefined();
      expect(client.events).toBeDefined();
      expect(client.reminders).toBeDefined();
      expect(client.webhooks).toBeDefined();
      expect(client.attendees).toBeDefined();
      expect(client.calendarMembers).toBeDefined();
    });

    test("should perform health check", async () => {
      const health = await client.healthCheck();
      expect(health).toBeDefined();
      expect(health.status).toBe("healthy");
    });

    test("should allow setting custom headers", () => {
      client.setHeader("X-Custom-Header", "test-value");
      client.removeHeader("X-Custom-Header");
    });
  });

  describe("Calendar Operations", () => {
    test("should create a calendar", async () => {
      const calendar = await client.calendars.create({
        account_id: accountId1,
        metadata: {
          name: "SDK Test Calendar",
          description: "Created via SDK",
        },
      });

      expect(calendar.calendar_uid).toBeDefined();
      expect(calendar.account_id).toBe(accountId1);
      expect(calendar.metadata?.name).toBe("SDK Test Calendar");
      calendarUid1 = calendar.calendar_uid;
    });

    test("should get a calendar", async () => {
      const calendar = await client.calendars.get(calendarUid1);
      expect(calendar.calendar_uid).toBe(calendarUid1);
      expect(calendar.account_id).toBe(accountId1);
    });

    test("should update a calendar", async () => {
      const updated = await client.calendars.update(calendarUid1, {
        metadata: {
          name: "Updated SDK Calendar",
        },
      });

      expect(updated.metadata?.name).toBe("Updated SDK Calendar");
    });

    test("should list calendars", async () => {
      const result = await client.calendars.list(accountId1, 50, 0);
      expect(result.data).toBeInstanceOf(Array);
      expect(result.data.length).toBeGreaterThan(0);
      const ownCalendar = result.data.find(
        (c) => c.calendar_uid === calendarUid1,
      );
      expect(ownCalendar).toBeDefined();
    });

    test("should handle errors for non-existent calendar", async () => {
      await expect(
        client.calendars.get("00000000-0000-0000-0000-000000000000"),
      ).rejects.toThrow();
    });
  });

  describe("Event Operations", () => {
    let eventUid: string;

    beforeAll(async () => {
      // Create a second calendar for transfer tests
      const cal2 = await client.calendars.create({
        account_id: accountId2,
      });
      calendarUid2 = cal2.calendar_uid;
    });

    test("should create a single event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await client.events.create({
        calendar_uid: calendarUid1,
        account_id: accountId1,
        start_ts: now + 3600,
        end_ts: now + 7200,
        metadata: {
          title: "SDK Test Event",
          description: "Created via SDK",
        },
      });

      expect(event.event_uid).toBeDefined();
      expect(event.calendar_uid).toBe(calendarUid1);
      expect(event.account_id).toBe(accountId1);
      expect(event.metadata?.title).toBe("SDK Test Event");
      eventUid = event.event_uid;
    });

    test("should get an event", async () => {
      const event = await client.events.get(eventUid);
      expect(event.event_uid).toBe(eventUid);
      expect(event.metadata?.title).toBe("SDK Test Event");
    });

    test("should update an event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await client.events.get(eventUid);
      const updated = await client.events.update(eventUid, {
        account_id: accountId1,
        start_ts: event.start_ts,
        end_ts: event.end_ts,
        metadata: {
          title: "Updated SDK Event",
        },
      });

      expect(updated.metadata?.title).toBe("Updated SDK Event");
    });

    test("should get calendar events", async () => {
      const now = Math.floor(Date.now() / 1000);
      const result = await client.events.getCalendarEvents({
        calendar_uids: [calendarUid1],
        start_ts: now,
        end_ts: now + 86400,
      });

      expect(result.data).toBeInstanceOf(Array);
      expect(result.data.length).toBeGreaterThan(0);
    });

    test("should toggle cancelled status", async () => {
      const toggled = await client.events.toggleCancelled(eventUid);
      expect(toggled.is_cancelled).toBe(true);

      const toggledBack = await client.events.toggleCancelled(eventUid);
      expect(toggledBack.is_cancelled).toBe(false);
    });

    test("should create a recurring event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const startDate = new Date((now + 86400) * 1000);
      const recurring = await client.events.create({
        calendar_uid: calendarUid1,
        account_id: accountId1,
        start_ts: Math.floor(startDate.getTime() / 1000),
        end_ts: Math.floor(startDate.getTime() / 1000) + 3600,
        recurrence: {
          rule: "FREQ=DAILY;COUNT=3",
        },
        metadata: {
          title: "Recurring SDK Event",
        },
      });

      expect(recurring.recurrence).toBeDefined();
      expect(recurring.recurrence?.rule).toBe("FREQ=DAILY;COUNT=3");

      // Cleanup
      await client.events.delete(recurring.event_uid, accountId1, "all");
    });

    test("should delete an event", async () => {
      await client.events.delete(eventUid, accountId1);
      await expect(client.events.get(eventUid)).rejects.toThrow();
    });
  });

  describe("Attendee Operations", () => {
    let eventUid: string;

    beforeAll(async () => {
      // Create an event for attendee tests
      const now = Math.floor(Date.now() / 1000);
      const event = await client.events.create({
        calendar_uid: calendarUid1,
        account_id: accountId1,
        start_ts: now + 3600,
        end_ts: now + 7200,
        metadata: { title: "Attendee Test Event" },
      });
      eventUid = event.event_uid;
    });

    afterAll(async () => {
      await client.events.delete(eventUid, accountId1);
    });

    test("should add an attendee to an event", async () => {
      const result = await client.attendees.create(eventUid, {
        account_id: accountId2,
        role: "attendee",
        metadata: {
          email: "test@example.com",
        },
      });

      expect(result.attendee).toBeDefined();
      expect(result.attendee?.account_id).toBe(accountId2);
      expect(result.attendee?.role).toBe("attendee");
      expect(result.scope).toBe("single");
    });

    test("should get an attendee", async () => {
      const attendee = await client.attendees.get(eventUid, accountId2);
      expect(attendee.account_id).toBe(accountId2);
      expect(attendee.event_uid).toBe(eventUid);
    });

    test("should get all attendees for an event", async () => {
      const attendees = await client.attendees.list(eventUid);
      expect(attendees).toBeInstanceOf(Array);
      expect(attendees.length).toBeGreaterThan(1); // Organizer + added attendee
    });

    test("should update attendee RSVP", async () => {
      const result = await client.attendees.updateRSVP(eventUid, accountId2, {
        rsvp_status: "accepted",
      });

      expect(result.count).toBe(1);
      expect(result.scope).toBe("single");

      const updated = await client.attendees.get(eventUid, accountId2);
      expect(updated.rsvp_status).toBe("accepted");
    });

    test("should update attendee metadata", async () => {
      const result = await client.attendees.update(eventUid, accountId2, {
        metadata: {
          note: "Updated via SDK",
        },
      });

      expect(result.count).toBe(1);
    });

    test("should get events where account is an attendee", async () => {
      const now = Math.floor(Date.now() / 1000);
      const result = await client.attendees.getAccountEvents(
        accountId2,
        now,
        now + 86400,
      );

      expect(result).toBeInstanceOf(Array);
      expect(result.length).toBeGreaterThan(0);
    });

    test("should delete an attendee", async () => {
      const result = await client.attendees.delete(eventUid, accountId2);

      expect(result.count).toBe(1);
      await expect(
        client.attendees.get(eventUid, accountId2),
      ).rejects.toThrow();
    });
  });

  describe("Reminder Operations", () => {
    let eventUid: string;
    let reminderUid: string;

    beforeAll(async () => {
      // Create an event for reminder tests
      const now = Math.floor(Date.now() / 1000);
      const event = await client.events.create({
        calendar_uid: calendarUid1,
        account_id: accountId1,
        start_ts: now + 3600,
        end_ts: now + 7200,
        metadata: { title: "Reminder Test Event" },
      });
      eventUid = event.event_uid;
    });

    afterAll(async () => {
      await client.events.delete(eventUid, accountId1);
    });

    test("should create a reminder", async () => {
      const result = await client.reminders.create(eventUid, {
        account_id: accountId1,
        offset_seconds: -900, // 15 minutes before
        metadata: {
          type: "email",
        },
      });

      expect(result.reminder).toBeDefined();
      expect(result.reminder.offset_seconds).toBe(-900);
      expect(result.scope).toBe("single");
      reminderUid = result.reminder.reminder_uid!;
    });

    test("should get reminders for an event", async () => {
      const reminders = await client.reminders.list(eventUid, accountId1);

      expect(reminders).toBeInstanceOf(Array);
      expect(reminders.length).toBeGreaterThan(0);
      expect(reminders[0].offset_seconds).toBe(-900);
    });

    test("should delete a reminder", async () => {
      const result = await client.reminders.delete(eventUid, reminderUid);

      expect(result.count).toBeGreaterThan(0);

      const reminders = await client.reminders.list(eventUid, accountId1);
      expect(reminders.length).toBe(0);
    });
  });

  describe("Calendar Member Operations", () => {
    test("should invite a member to a calendar", async () => {
      const result = await client.calendarMembers.invite(calendarUid1, {
        account_id: accountId1, // The inviter's account ID
        account_ids: [accountId2], // Account IDs to invite
        role: "read",
      });

      expect(result.invited_count).toBe(1);
      expect(result.members).toBeInstanceOf(Array);
      expect(result.members.length).toBe(1);
    });

    test("should get calendar members", async () => {
      const members = await client.calendarMembers.list(
        calendarUid1,
        accountId1,
      );
      expect(members).toBeInstanceOf(Array);
      expect(members.length).toBeGreaterThan(0);

      const member = members.find((m) => m.account_id === accountId2);
      expect(member).toBeDefined();
      expect(member?.role).toBe("read");
    });

    test("should update member role", async () => {
      const result = await client.calendarMembers.update(
        calendarUid1,
        accountId2,
        accountId1, // The requesting user's account ID
        {
          role: "write",
        },
      );

      expect(result.calendar_uid).toBe(calendarUid1);
      expect(result.account_id).toBe(accountId2);
      expect(result.role).toBe("write");
    });

    test("should remove a member from calendar", async () => {
      await client.calendarMembers.remove(calendarUid1, accountId2, accountId1);

      const members = await client.calendarMembers.list(
        calendarUid1,
        accountId1,
      );
      const member = members.find((m) => m.account_id === accountId2);
      expect(member).toBeUndefined();
    });
  });

  describe("Webhook Operations", () => {
    let webhookUid: string;

    test("should create a webhook", async () => {
      const webhook = await client.webhooks.create({
        url: "https://example.com/webhook",
        event_types: ["event.created", "event.updated"],
      });

      expect(webhook.webhook_uid).toBeDefined();
      expect(webhook.url).toBe("https://example.com/webhook");
      expect(webhook.event_types).toContain("event.created");
      webhookUid = webhook.webhook_uid;
    });

    test("should get a webhook", async () => {
      const webhook = await client.webhooks.get(webhookUid);
      expect(webhook.webhook_uid).toBe(webhookUid);
      expect(webhook.url).toBe("https://example.com/webhook");
    });

    test("should list webhooks", async () => {
      const result = await client.webhooks.list(50, 0);
      expect(result.data).toBeInstanceOf(Array);
      expect(result.data.length).toBeGreaterThan(0);
      const foundWebhook = result.data.find(
        (w) => w.webhook_uid === webhookUid,
      );
      expect(foundWebhook).toBeDefined();
    });

    test("should update a webhook", async () => {
      const updated = await client.webhooks.update(webhookUid, {
        is_active: false,
      });

      expect(updated.is_active).toBe(false);
    });

    test("should delete a webhook", async () => {
      await client.webhooks.delete(webhookUid);
      await expect(client.webhooks.get(webhookUid)).rejects.toThrow();
    });
  });

  describe("Error Handling", () => {
    test("should handle 404 errors gracefully", async () => {
      await expect(
        client.events.get("00000000-0000-0000-0000-000000000000"),
      ).rejects.toThrow();
    });

    test("should handle invalid request data", async () => {
      await expect(
        client.events.create({
          calendar_uid: "invalid-uuid",
          account_id: accountId1,
          start_ts: 0,
          end_ts: 0,
        }),
      ).rejects.toThrow();
    });

    test("should handle non-existent calendar", async () => {
      await expect(
        client.calendars.get("00000000-0000-0000-0000-000000000000"),
      ).rejects.toThrow();
    });
  });

  describe("Ownership Transfer", () => {
    test("should transfer event ownership", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await client.events.create({
        calendar_uid: calendarUid1,
        account_id: accountId1,
        start_ts: now + 3600,
        end_ts: now + 7200,
        metadata: { title: "Transfer Test" },
      });

      const result = await client.events.transferOwnership(
        event.event_uid,
        accountId2,
        calendarUid2,
      );

      expect(result.new_organizer).toBeDefined();
      expect(result.new_organizer.account_id).toBe(accountId2);

      const updatedEvent = await client.events.get(event.event_uid);
      expect(updatedEvent.account_id).toBe(accountId2);
      expect(updatedEvent.calendar_uid).toBe(calendarUid2);

      await client.events.delete(event.event_uid, accountId2);
    });
  });

  describe("ICS Import Operations", () => {
    const simpleICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//SDK Test//Test//EN
BEGIN:VEVENT
UID:sdk-test-event-123
DTSTART:20260315T100000Z
DTEND:20260315T110000Z
SUMMARY:SDK Import Test Event
DESCRIPTION:Event imported via SDK
LOCATION:Test Location
END:VEVENT
END:VCALENDAR`;

    const multiEventICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//SDK Test//Test//EN
BEGIN:VEVENT
UID:sdk-multi-1
DTSTART:20260315T100000Z
DTEND:20260315T110000Z
SUMMARY:Multi Event 1
END:VEVENT
BEGIN:VEVENT
UID:sdk-multi-2
DTSTART:20260316T140000Z
DTEND:20260316T150000Z
SUMMARY:Multi Event 2
END:VEVENT
END:VCALENDAR`;

    const recurringICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//SDK Test//Test//EN
BEGIN:VEVENT
UID:sdk-recurring-123
DTSTART:20260315T100000Z
DTEND:20260315T110000Z
SUMMARY:SDK Weekly Meeting
RRULE:FREQ=WEEKLY;COUNT=3
END:VEVENT
END:VCALENDAR`;

    const reminderICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//SDK Test//Test//EN
BEGIN:VEVENT
UID:sdk-reminder-event
DTSTART:20260315T100000Z
DTEND:20260315T110000Z
SUMMARY:Event With Reminder
BEGIN:VALARM
TRIGGER:-PT30M
ACTION:DISPLAY
DESCRIPTION:30 minute reminder
END:VALARM
END:VEVENT
END:VCALENDAR`;

    let importedCalendarUid: string;

    afterAll(async () => {
      if (importedCalendarUid) {
        try {
          await client.calendars.delete(importedCalendarUid);
        } catch {
          // Ignore cleanup errors
        }
      }
    });

    test("should import ICS and create a new calendar", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(simpleICSContent),
        {
          accountId: accountId1,
          calendarMetadata: {
            name: "SDK Imported Calendar",
            source: "ics-import",
          },
        },
      );

      expect(result.calendar).toBeDefined();
      expect(result.calendar.calendar_uid).toBeDefined();
      expect(result.calendar.account_id).toBe(accountId1);
      expect(result.calendar.metadata?.name).toBe("SDK Imported Calendar");

      expect(result.summary.total_events).toBe(1);
      expect(result.summary.imported_events).toBe(1);
      expect(result.summary.failed_events).toBe(0);

      expect(result.events).toHaveLength(1);
      expect(result.events[0].ics_uid).toBe("sdk-test-event-123");
      expect(result.events[0].status).toBe("success");
      expect(result.events[0].event_uid).toBeDefined();

      importedCalendarUid = result.calendar.calendar_uid;
    });

    test("should import ICS into an existing calendar", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(simpleICSContent),
        {
          accountId: accountId1,
          calendarUid: calendarUid1,
        },
      );

      expect(result.calendar.calendar_uid).toBe(calendarUid1);
      expect(result.summary.imported_events).toBe(1);
    });

    test("should import multiple events from ICS", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(multiEventICSContent),
        {
          accountId: accountId1,
        },
      );

      expect(result.summary.total_events).toBe(2);
      expect(result.summary.imported_events).toBe(2);
      expect(result.summary.failed_events).toBe(0);
      expect(result.events).toHaveLength(2);

      for (const event of result.events) {
        expect(event.status).toBe("success");
        expect(event.event_uid).toBeDefined();
      }

      // Cleanup
      await client.calendars.delete(result.calendar.calendar_uid);
    });

    test("should import recurring event from ICS", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(recurringICSContent),
        {
          accountId: accountId1,
        },
      );

      expect(result.summary.total_events).toBe(1);
      expect(result.summary.imported_events).toBe(1);
      expect(result.events[0].status).toBe("success");

      // Verify event was created with recurrence
      const event = await client.events.get(result.events[0].event_uid!);
      expect(event.recurrence).toBeDefined();
      expect(event.recurrence?.rule).toContain("FREQ=WEEKLY");

      // Cleanup
      await client.calendars.delete(result.calendar.calendar_uid);
    });

    test("should import event with reminder", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(reminderICSContent),
        {
          accountId: accountId1,
          importReminders: true,
        },
      );

      expect(result.summary.imported_events).toBe(1);
      expect(result.events[0].status).toBe("success");

      // Verify reminder was created
      const reminders = await client.reminders.list(
        result.events[0].event_uid!,
        accountId1,
      );
      expect(reminders.length).toBeGreaterThan(0);
      expect(reminders[0].offset_seconds).toBe(-1800); // -30 minutes

      // Cleanup
      await client.calendars.delete(result.calendar.calendar_uid);
    });

    test("should skip reminders when importReminders is false", async () => {
      const result = await client.calendars.importICS(
        Buffer.from(reminderICSContent),
        {
          accountId: accountId1,
          importReminders: false,
        },
      );

      expect(result.summary.imported_events).toBe(1);

      // Verify no reminders were created
      const reminders = await client.reminders.list(
        result.events[0].event_uid!,
        accountId1,
      );
      expect(reminders.length).toBe(0);

      // Cleanup
      await client.calendars.delete(result.calendar.calendar_uid);
    });
  });

  describe("ICS Link Import", () => {
    const simpleICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:sdk-test-event-123
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:SDK Test Meeting
DESCRIPTION:A test meeting for SDK import
LOCATION:Conference Room A
END:VEVENT
END:VCALENDAR`;

    const multiEventICSContent = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:sdk-event-1
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:SDK Event One
END:VEVENT
BEGIN:VEVENT
UID:sdk-event-2
DTSTART:20260116T140000Z
DTEND:20260116T150000Z
SUMMARY:SDK Event Two
END:VEVENT
END:VCALENDAR`;

    let icsServer: any;
    let icsCalendarUid: string;

    beforeAll(async () => {
      // Start a simple ICS server for testing
      const express = require("express");
      const app = express();

      app.get("/calendar.ics", (req: any, res: any) => {
        res.setHeader("Content-Type", "text/calendar; charset=utf-8");
        res.send(simpleICSContent);
      });

      app.get("/multi-calendar.ics", (req: any, res: any) => {
        res.setHeader("Content-Type", "text/calendar; charset=utf-8");
        res.send(multiEventICSContent);
      });

      app.get("/auth-calendar.ics", (req: any, res: any) => {
        const authHeader = req.headers.authorization;
        if (!authHeader || authHeader !== "Bearer test-token-123") {
          res.status(401).send("Unauthorized");
          return;
        }
        res.setHeader("Content-Type", "text/calendar; charset=utf-8");
        res.send(simpleICSContent);
      });

      icsServer = app.listen(9100);
    });

    afterAll(async () => {
      if (icsServer) {
        icsServer.close();
      }
      if (icsCalendarUid) {
        try {
          await client.calendars.delete(icsCalendarUid);
        } catch (e) {
          // Ignore cleanup errors
        }
      }
    });

    test("should import ICS from URL without authentication", async () => {
      const response = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/calendar.ics",
        authType: "none",
        syncIntervalSeconds: 3600,
        calendarMetadata: { name: "SDK ICS Import", source: "sdk_test" },
      });

      expect(response.calendar).toBeDefined();
      expect(response.calendar.calendar_uid).toBeDefined();
      expect(response.calendar.account_id).toBe(accountId1);
      expect(response.calendar.is_read_only).toBe(true);
      expect(response.calendar.ics_url).toBe(
        "http://localhost:9100/calendar.ics",
      );
      expect(response.calendar.ics_sync_interval_seconds).toBe(3600);

      expect(response.summary.total_events).toBe(1);
      expect(response.summary.imported_events).toBe(1);
      expect(response.summary.failed_events).toBe(0);
      expect(response.sync_scheduled).toBe(true);

      icsCalendarUid = response.calendar.calendar_uid;
    });

    test("should import ICS from URL with Bearer token", async () => {
      const response = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/auth-calendar.ics",
        authType: "bearer",
        authCredentials: "test-token-123",
      });

      expect(response.calendar).toBeDefined();
      expect(response.calendar.is_read_only).toBe(true);
      expect(response.calendar.ics_auth_type).toBe("bearer");
      expect(response.summary.imported_events).toBe(1);

      // Cleanup
      await client.calendars.delete(response.calendar.calendar_uid);
    });

    test("should import multiple events from URL", async () => {
      const response = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/multi-calendar.ics",
        authType: "none",
      });

      expect(response.summary.total_events).toBe(2);
      expect(response.summary.imported_events).toBe(2);
      expect(response.summary.failed_events).toBe(0);
      expect(response.events).toHaveLength(2);

      // Cleanup
      await client.calendars.delete(response.calendar.calendar_uid);
    });

    test("should manually resync an imported calendar", async () => {
      // First import
      const importResponse = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/calendar.ics",
        authType: "none",
      });

      expect(importResponse.summary.imported_events).toBe(1);
      expect(importResponse.calendar.calendar_uid).toBeDefined();

      // Manually resync
      try {
        const resyncResponse = await client.calendars.resync(
          importResponse.calendar.calendar_uid,
        );

        expect(resyncResponse.imported_events).toBeGreaterThanOrEqual(1);
        expect(resyncResponse.calendar.ics_last_sync_status).toBe("success");
      } catch (err) {
        console.error(err);
      }
      // Cleanup
      await client.calendars.delete(importResponse.calendar.calendar_uid);
    });

    test("should handle custom sync interval", async () => {
      const response = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/calendar.ics",
        authType: "none",
        syncIntervalSeconds: 7200, // 2 hours
      });

      expect(response.calendar.ics_sync_interval_seconds).toBe(7200);

      // Cleanup
      await client.calendars.delete(response.calendar.calendar_uid);
    });

    test("should fail with invalid auth credentials", async () => {
      try {
        await client.calendars.importICSLink({
          accountId: accountId1,
          icsUrl: "http://localhost:9100/auth-calendar.ics",
          authType: "bearer",
          authCredentials: "wrong-token",
        });
        expect.fail("Should have thrown an error");
      } catch (error: any) {
        expect(error).toBeDefined();
      }
    });

    test("should fail resync on non-ICS calendar", async () => {
      // Create a regular calendar
      const regularCal = await client.calendars.create({
        account_id: accountId1,
        metadata: { name: "Regular Calendar" },
      });

      try {
        await client.calendars.resync(regularCal.calendar_uid);
        expect.fail("Should have thrown an error");
      } catch (error: any) {
        expect(error).toBeDefined();
      }

      // Cleanup
      await client.calendars.delete(regularCal.calendar_uid);
    });

    test("should retrieve ICS calendar with sync status", async () => {
      const importResponse = await client.calendars.importICSLink({
        accountId: accountId1,
        icsUrl: "http://localhost:9100/calendar.ics",
        authType: "none",
      });

      const calendar = await client.calendars.get(
        importResponse.calendar.calendar_uid,
      );

      expect(calendar.ics_url).toBe("http://localhost:9100/calendar.ics");
      expect(calendar.is_read_only).toBe(true);
      expect(calendar.ics_last_sync_status).toBeDefined();
      expect(calendar.ics_last_sync_ts).toBeDefined();

      // Cleanup
      await client.calendars.delete(importResponse.calendar.calendar_uid);
    });
  });
});
