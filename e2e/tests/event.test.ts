import { beforeAll, describe, expect, test } from "vitest";
import {
  createEvent,
  deleteEvent,
  getCalendarEvents,
  getEvent,
  toggleCancelledStatusEvent,
  updateEvent,
} from "../helpers/event";
import { createCalendar } from "../helpers/calendar";
import {
  ErrorObject,
  EventObject,
  CalendarObject,
  GenericPagedResponse,
  SuccessObject,
} from "../helpers/types";
import { generationWindow } from "../constants/event";

describe("Single Event API", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;
  let eventUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountId);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create an event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const eventResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      metadata: { title: "Test Event" },
    })) as EventObject;
    expect(eventResponse.calendar_uid).toBe(calendarUid);
    eventUid = eventResponse.event_uid;
  });

  test("Should not create an event with invalid calendar UID", async () => {
    const eventResponse = (await createEvent({
      calendarUid: "invalid-calendar-uid",
      accountId,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
    })) as ErrorObject;

    expect(eventResponse.error).toBe("Invalid request body");
    expect(eventResponse.details).toBe("invalid UUID length: 20");
  });

  test("Should not create an event with valid calendar UID but calendar does not exist", async () => {
    const invalidCalendarResponse = (await createEvent({
      calendarUid: crypto.randomUUID(),
      accountId,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
    })) as ErrorObject;
    expect(invalidCalendarResponse.error).toBe("Calendar not found");
  });

  test("Should not create an event with invalid start and end timestamps", async () => {
    const invalidStartAndEndResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs: 0,
      endTs: 0,
      metadata: { title: "Test Event" },
    })) as ErrorObject;

    expect(invalidStartAndEndResponse.error).toBe(
      "start_ts is required and must be positive"
    );
    expect(invalidStartAndEndResponse.details).toBe(
      "start_ts is required and must be positive"
    );

    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const invalidEndAndStartResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs: 0,
      metadata: { title: "Test Event" },
    })) as ErrorObject;
    expect(invalidEndAndStartResponse.error).toBe(
      "end_ts is required and must be positive"
    );
    expect(invalidEndAndStartResponse.details).toBe(
      "end_ts is required and must be positive"
    );

    const invalidEndAndStartResponse2 = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs: startTs - 3600,
      metadata: { title: "Test Event" },
    })) as ErrorObject;
    expect(invalidEndAndStartResponse2.error).toBe(
      "end_ts must be greater than start_ts"
    );
  });

  test("SHould not create an event that more than 24 hours", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600 * 25;
    const eventResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      metadata: { title: "Test Event" },
    })) as ErrorObject;
    expect(eventResponse.error).toBe(
      "Event duration cannot be more than 24 hours"
    );
  });

  test("Should get an event", async () => {
    const eventResponse = (await getEvent({ eventUid })) as EventObject;
    expect(eventResponse.calendar_uid).toBe(calendarUid);
    expect(eventResponse.event_uid).toBe(eventUid);
  });

  test("Should update an event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const eventResponse = (await updateEvent({
      eventUid,
      startTs,
      endTs,
      accountId,
      scope: "single",
      metadata: { title: "Updated Event" },
    })) as EventObject;
    expect(eventResponse.event_uid).toBe(eventUid);
    expect(eventResponse.calendar_uid).toBe(calendarUid);
    expect(eventResponse.start_ts).toBe(startTs);
    expect(eventResponse.end_ts).toBe(endTs);
    expect(eventResponse.metadata).toEqual({ title: "Updated Event" });
  });

  test("Should not update an event with invalid start and end timestamps", async () => {
    const invalidStartAndEndResponse = (await updateEvent({
      eventUid,
      accountId,
      startTs: 0,
      endTs: 0,
      scope: "single",
    })) as ErrorObject;
    expect(invalidStartAndEndResponse.error).toBe(
      "start_ts is required and must be positive"
    );

    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;

    const invalidEndAndStartResponse = (await updateEvent({
      eventUid,
      accountId,
      startTs,
      endTs: 0,
      scope: "single",
    })) as ErrorObject;
    expect(invalidEndAndStartResponse.error).toBe(
      "end_ts is required and must be positive"
    );

    const invalidEndAndStartResponse2 = (await updateEvent({
      eventUid,
      accountId,
      startTs,
      endTs: startTs - 3600,
      scope: "single",
    })) as ErrorObject;
    expect(invalidEndAndStartResponse2.error).toBe(
      "end_ts must be greater than start_ts"
    );

    const invalidEndAndStartResponse3 = (await updateEvent({
      eventUid,
      startTs: 0,
      accountId,
      endTs: startTs,
      scope: "single",
    })) as ErrorObject;
    expect(invalidEndAndStartResponse3.error).toBe(
      "start_ts is required and must be positive"
    );
  });

  test("Should cancel an event", async () => {
    const response = (await toggleCancelledStatusEvent({
      eventUid,
    })) as EventObject;
    expect(response.event_uid).toBe(eventUid);
    expect(response.is_cancelled).toBe(true);

    // toggle again to un-cancel
    const response2 = (await toggleCancelledStatusEvent({
      eventUid,
    })) as EventObject;
    expect(response2.event_uid).toBe(eventUid);
    expect(response2.is_cancelled).toBe(false);
  });

  test("Should delete an event", async () => {
    const response = (await deleteEvent({
      eventUid,
      accountId,
    })) as SuccessObject;
    expect(response.success).toBe(true);

    const getResponse = (await getEvent({
      eventUid,
    })) as ErrorObject;
    expect(getResponse.error).toBe("Event not found");
  });
});

describe("Recurring Event API", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;
  let masterEventUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountId);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create a recurring event", async () => {
    const response = (await createEvent({
      calendarUid: calendarUid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
      accountId,
    })) as EventObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.event_uid).toBeDefined();
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(response.metadata).toEqual({ title: "Test Event" });

    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 30,
    })) as GenericPagedResponse<EventObject>;
    // List returns only instances (one per occurrence), not the master
    expect(getResponse.count).toBe(10);
    expect(getResponse.data).toHaveLength(10);
    expect(getResponse.data[0].calendar_uid).toBe(calendarUid);
    // Create response returns the master; list returns only instances
    masterEventUid = response.event_uid;
    const firstInstance = getResponse.data[0];
    expect(firstInstance?.master_event_uid).toBe(masterEventUid);
    expect(firstInstance?.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(firstInstance?.metadata).toEqual({ title: "Test Event" });
  });

  test("Update single instance of recurring event", async () => {
    const response = (await updateEvent({
      eventUid: masterEventUid,
      accountId,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      scope: "single",
      metadata: { title: "Updated Event" },
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
    })) as EventObject;
    expect(response.event_uid).toBe(masterEventUid);
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.start_ts).toBe(Math.floor(Date.now() / 1000) + 3600);
    expect(response.end_ts).toBe(Math.floor(Date.now() / 1000) + 3600 + 3600);
    expect(response.metadata).toEqual({ title: "Updated Event" });

    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 60,
    })) as GenericPagedResponse<EventObject>;
    expect(getResponse.count).toBe(10);
    expect(getResponse.data).toHaveLength(10);
    expect(getResponse.data[0].calendar_uid).toBe(calendarUid);
    // Update with scope "single" on master only updates the master record; list returns instances only (unchanged)
    expect(getResponse.data.every((e) => e.master_event_uid === masterEventUid)).toBe(true);
    expect(getResponse.data.every((e) => e.metadata?.title === "Test Event")).toBe(true);
  });

  test("Update entire series based on the master event of recurring event", async () => {
    const response = (await updateEvent({
      eventUid: masterEventUid,
      accountId,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      scope: "all",
      metadata: { title: "Updated Event" },
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
    })) as EventObject;
    expect(response.event_uid).toBe(masterEventUid);
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.start_ts).toBe(Math.floor(Date.now() / 1000) + 3600);
    expect(response.end_ts).toBe(Math.floor(Date.now() / 1000) + 3600 + 3600);
    expect(response.metadata).toEqual({ title: "Updated Event" });
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(response.is_recurring_instance).toBe(false);

    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 60,
    })) as GenericPagedResponse<EventObject>;
    expect(getResponse.count).toBe(10);
    expect(getResponse.data).toHaveLength(10);
    for (const event of getResponse.data) {
      expect(event.calendar_uid).toBe(calendarUid);
      expect(event.metadata).toEqual({ title: "Updated Event" });
    }
  });
});

describe("Recurring Event API - On-Demand Expansion", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;
  let masterEventUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountId);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create a recurring event with on-demand expansion", async () => {
    const response = (await createEvent({
      calendarUid: calendarUid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
      recurrence: { rule: "FREQ=WEEKLY" },
      accountId,
    })) as EventObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.event_uid).toBeDefined();
    expect(response.recurrence).toEqual({ rule: "FREQ=WEEKLY" });
    expect(response.metadata).toEqual({ title: "Test Event" });
    masterEventUid = response.event_uid;
  });

  test("Should get events beyond the generation window (2 years from now)", async () => {
    const tenYearsFromNow = Math.floor(Date.now() / 1000) + 86400 * 365 * 10;
    const tenYearsFromNowPlus30Days = tenYearsFromNow + 86400 * 30;
    const response = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: tenYearsFromNow,
      endTs: tenYearsFromNowPlus30Days,
    })) as GenericPagedResponse<EventObject>;
    expect(response.count).toBe(4);
    expect(response.data).toHaveLength(4);
    for (const event of response.data) {
      expect(event.start_ts).toBeGreaterThan(tenYearsFromNow);
      expect(event.end_ts).toBeLessThan(tenYearsFromNowPlus30Days);
    }
  });

  test("Should be able to edit events beyond the generation window", async () => {
    const startTs = Math.floor(Date.now() / 1000) + generationWindow;
    const endTs = startTs + 86400 * 30;
    const response = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs,
      endTs,
    })) as GenericPagedResponse<EventObject>;
    expect(response.count).toBe(4);
    expect(response.data).toHaveLength(4);

    const firstEvent = response.data[0] as EventObject;
    expect(firstEvent.event_uid).toBe(masterEventUid);

    const updateResponse = (await updateEvent({
      eventUid: firstEvent.event_uid,
      accountId,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      scope: "single",
      metadata: { title: "Updated Event" },
    })) as EventObject;
    expect(updateResponse.event_uid).toBe(firstEvent.event_uid);
    expect(updateResponse.start_ts).toBe(Math.floor(Date.now() / 1000) + 3600);
    expect(updateResponse.end_ts).toBe(
      Math.floor(Date.now() / 1000) + 3600 + 3600
    );
    expect(updateResponse.metadata).toEqual({ title: "Updated Event" });
    expect(updateResponse.recurrence).toEqual({ rule: "FREQ=WEEKLY" });
    expect(updateResponse.is_recurring_instance).toBe(false);
  });
});

describe("Timezone Support API", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create an event with timezone", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const eventResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "America/New_York",
      metadata: { title: "Timezone Event" },
    })) as EventObject;

    expect(eventResponse.calendar_uid).toBe(calendarUid);
    expect(eventResponse.event_uid).toBeDefined();
    expect(eventResponse.timezone).toBe("America/New_York");
    // local_start should be computed from start_ts + timezone
    expect(eventResponse.local_start).toBeDefined();
  });

  test("Should create an event with timezone and local_start", async () => {
    // Create an event at 9am local time in New York, 1 hour duration
    // Use a date relative to now to avoid timestamp conflicts
    const futureDate = new Date();
    futureDate.setDate(futureDate.getDate() + 7); // 7 days from now
    futureDate.setHours(9, 0, 0, 0);
    const localStart = futureDate.toISOString().slice(0, 19); // "YYYY-MM-DDTHH:MM:SS"

    // Compute approximate start_ts and end_ts (these will be adjusted by the API)
    const approxStartTs = Math.floor(futureDate.getTime() / 1000);
    const endTs = approxStartTs + 3600; // 1 hour duration

    const eventResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs: approxStartTs,
      endTs,
      timezone: "America/New_York",
      localStart,
      metadata: { title: "Local Time Event" },
    })) as EventObject;

    expect(eventResponse.calendar_uid).toBe(calendarUid);
    expect(eventResponse.timezone).toBe("America/New_York");
    expect(eventResponse.local_start).toBe(localStart);
    // The start_ts should be computed from local_start in America/New_York timezone
    expect(eventResponse.start_ts).toBeDefined();
  });

  test("Should reject invalid timezone", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const eventResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Invalid/Timezone",
      metadata: { title: "Invalid Timezone Event" },
    })) as ErrorObject;

    expect(eventResponse.error).toBe(
      "Invalid timezone: must be a valid IANA timezone (e.g., America/New_York)"
    );
  });

  test("Should update event timezone", async () => {
    // First create an event
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const createResponse = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "America/New_York",
      metadata: { title: "Original Timezone" },
    })) as EventObject;

    expect(createResponse.timezone).toBe("America/New_York");

    // Update to different timezone
    const updateResponse = (await updateEvent({
      eventUid: createResponse.event_uid,
      accountId,
      startTs,
      endTs,
      timezone: "America/Los_Angeles",
      scope: "single",
      metadata: { title: "Updated Timezone" },
    })) as EventObject;

    expect(updateResponse.timezone).toBe("America/Los_Angeles");
    expect(updateResponse.local_start).toBeDefined();
  });
});

describe("Timezone Recurring Events", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create recurring event with timezone", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "America/New_York",
      recurrence: { rule: "FREQ=DAILY;COUNT=5" },
      metadata: { title: "Daily Recurring with TZ" },
    })) as EventObject;

    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.timezone).toBe("America/New_York");
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=5" });

    // Get all instances
    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: now,
      endTs: now + 86400 * 30,
    })) as GenericPagedResponse<EventObject>;

    // List returns only instances (one per occurrence), not the master
    expect(getResponse.count).toBe(5);

    // All instances should inherit timezone
    for (const event of getResponse.data) {
      expect(event.timezone).toBe("America/New_York");
      if (event.is_recurring_instance) {
        expect(event.local_start).toBeDefined();
      }
    }
  });

  test("Should create recurring event with local_start for DST-aware scheduling", async () => {
    // Create a daily recurring event with timezone (America/New_York)
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600; // 1 hour duration

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "America/New_York",
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
      metadata: { title: "DST Test Event" },
    })) as EventObject;

    expect(response.timezone).toBe("America/New_York");
    expect(response.local_start).toBeDefined();
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });

    // Get all instances
    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: now,
      endTs: now + 86400 * 30,
    })) as GenericPagedResponse<EventObject>;

    // List returns only instances; use first instance's local time as reference
    const instances = getResponse.data.filter((e) => e.is_recurring_instance);
    expect(instances.length).toBeGreaterThan(0);
    const firstLocalStart = instances[0]?.local_start;
    expect(firstLocalStart).toBeDefined();
    const expectedLocalTime = firstLocalStart?.split("T")[1]; // e.g., "14:00:00"

    // All recurring instances should maintain the same local time (DST-aware scheduling)
    for (const event of instances) {
      if (event.local_start) {
        const instanceLocalTime = event.local_start.split("T")[1];
        expect(instanceLocalTime).toBe(expectedLocalTime);
      }
    }
  });
});

describe("Timezone Edge Cases", () => {
  let accountId: string = crypto.randomUUID();
  let calendarUid: string;

  beforeAll(async () => {
    const calendarResponse = (await createCalendar({
      accountId: accountId,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should handle half-hour offset timezone (Asia/Kolkata UTC+5:30)", async () => {
    // India Standard Time is UTC+5:30 - not a full hour offset
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Asia/Kolkata",
      metadata: { title: "India Time Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Asia/Kolkata");
    expect(response.local_start).toBeDefined();
    // Verify the event was created successfully with the unusual offset
    expect(response.start_ts).toBe(startTs);
  });

  test("Should handle 45-minute offset timezone (Asia/Kathmandu UTC+5:45)", async () => {
    // Nepal Time is UTC+5:45 - one of the few 45-minute offset timezones
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Asia/Kathmandu",
      metadata: { title: "Nepal Time Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Asia/Kathmandu");
    expect(response.local_start).toBeDefined();
  });

  test("Should handle negative half-hour offset (Canada/Newfoundland UTC-3:30)", async () => {
    // Newfoundland is UTC-3:30 (or UTC-2:30 during DST)
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Canada/Newfoundland",
      metadata: { title: "Newfoundland Time Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Canada/Newfoundland");
    expect(response.local_start).toBeDefined();
  });

  test("Should handle far east timezone near date line (Pacific/Auckland UTC+12/+13)", async () => {
    // New Zealand - one of the first to see each new day
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Pacific/Auckland",
      metadata: { title: "New Zealand Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Pacific/Auckland");
    expect(response.local_start).toBeDefined();
  });

  test("Should handle far west timezone (Pacific/Honolulu UTC-10)", async () => {
    // Hawaii - no DST, always UTC-10
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Pacific/Honolulu",
      metadata: { title: "Hawaii Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Pacific/Honolulu");
    expect(response.local_start).toBeDefined();
  });

  test("Should create recurring event with half-hour offset timezone", async () => {
    // Test recurring events with India's UTC+5:30 timezone
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Asia/Kolkata",
      recurrence: { rule: "FREQ=DAILY;COUNT=5" },
      metadata: { title: "Daily India Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Asia/Kolkata");
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=5" });

    // Get all instances
    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: now,
      endTs: now + 86400 * 30,
    })) as GenericPagedResponse<EventObject>;

    // All instances should have the same local time
    const instances = getResponse.data.filter(
      (e) => e.is_recurring_instance && e.timezone === "Asia/Kolkata"
    );
    expect(instances.length).toBeGreaterThan(0);

    const firstInstanceTime = instances[0]?.local_start?.split("T")[1];
    for (const instance of instances) {
      expect(instance.local_start?.split("T")[1]).toBe(firstInstanceTime);
    }
  });

  test("Should handle multiple events in different timezones on same calendar", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 7200; // 2 hours from now
    const endTs = startTs + 3600;

    // Create events in multiple timezones
    const timezones = [
      "America/New_York", // UTC-5/-4
      "Europe/London", // UTC+0/+1
      "Asia/Tokyo", // UTC+9 (no DST)
      "Australia/Sydney", // UTC+10/+11 (DST in opposite season)
      "Asia/Kolkata", // UTC+5:30
    ];

    const events: EventObject[] = [];
    for (const tz of timezones) {
      const response = (await createEvent({
        calendarUid,
        accountId,
        startTs,
        endTs,
        timezone: tz,
        metadata: { title: `Event in ${tz}` },
      })) as EventObject;

      expect(response.timezone).toBe(tz);
      expect(response.local_start).toBeDefined();
      events.push(response);
    }

    // All events should have the same UTC start_ts
    for (const event of events) {
      expect(event.start_ts).toBe(startTs);
    }

    // But local_start should be different for each timezone
    const localStarts = events.map((e) => e.local_start);
    const uniqueLocalStarts = new Set(localStarts);
    // Should have 5 unique local_start values (one per timezone)
    expect(uniqueLocalStarts.size).toBe(5);
  });

  test("Should handle timezone with unusual DST rules (Australia/Lord_Howe UTC+10:30/+11)", async () => {
    // Lord Howe Island has a 30-minute DST shift (unique in the world)
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Australia/Lord_Howe",
      metadata: { title: "Lord Howe Island Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Australia/Lord_Howe");
    expect(response.local_start).toBeDefined();
  });

  test("Should handle recurring event across Southern Hemisphere DST (Australia/Sydney)", async () => {
    // Australia has DST from October to April (opposite of Northern Hemisphere)
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Australia/Sydney",
      recurrence: { rule: "FREQ=WEEKLY;COUNT=8" },
      metadata: { title: "Sydney Weekly Meeting" },
    })) as EventObject;

    expect(response.timezone).toBe("Australia/Sydney");
    expect(response.recurrence).toEqual({ rule: "FREQ=WEEKLY;COUNT=8" });

    // Get instances
    const getResponse = (await getCalendarEvents({
      calendarUids: calendarUid,
      startTs: now,
      endTs: now + 86400 * 60,
    })) as GenericPagedResponse<EventObject>;

    const sydneyInstances = getResponse.data.filter(
      (e) => e.is_recurring_instance && e.timezone === "Australia/Sydney"
    );

    // All instances should maintain the same local time
    if (sydneyInstances.length > 1) {
      const firstTime = sydneyInstances[0]?.local_start?.split("T")[1];
      for (const instance of sydneyInstances) {
        expect(instance.local_start?.split("T")[1]).toBe(firstTime);
      }
    }
  });

  test("Should handle UTC timezone explicitly", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "UTC",
      metadata: { title: "UTC Event" },
    })) as EventObject;

    expect(response.timezone).toBe("UTC");
    expect(response.local_start).toBeDefined();
    // For UTC, the local_start should directly correspond to the UTC timestamp
  });

  test("Should handle timezone near International Date Line (Pacific/Fiji UTC+12/+13)", async () => {
    // Fiji observes DST and is near the date line
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Pacific/Fiji",
      metadata: { title: "Fiji Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Pacific/Fiji");
    expect(response.local_start).toBeDefined();
  });

  test("Should handle timezone west of date line (Pacific/Samoa UTC+13/+14)", async () => {
    // Samoa switched sides of the date line in 2011, now UTC+13/+14
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId,
      startTs,
      endTs,
      timezone: "Pacific/Apia",
      metadata: { title: "Samoa Event" },
    })) as EventObject;

    expect(response.timezone).toBe("Pacific/Apia");
    expect(response.local_start).toBeDefined();
  });
});
