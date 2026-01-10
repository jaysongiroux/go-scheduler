import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  createEvent,
  deleteEvent,
  getCalendarEvents,
  getEvent,
  toggleCancelledStatusEvent,
  updateEvent,
} from "../helpers/event";
import { createAccount, deleteAccount } from "../helpers/account";
import { createCalendar } from "../helpers/calendar";
import {
  AccountObject,
  SuccessObject,
  ErrorObject,
  EventObject,
  CalendarObject,
  GenericPagedResponse,
} from "../helpers/types";
import { generationWindow } from "../constants/event";

describe("Single Event API", () => {
  const accounts: string[] = [];
  let calendarUid: string;
  let eventUid: string;

  afterAll(async () => {
    // deleting accounts will cascade to delete calendars and events
    for (const account of accounts) {
      const response = (await deleteAccount({
        accountId: account,
      })) as SuccessObject;
      expect(response.success).toBe(true);
    }
  });

  beforeAll(async () => {
    const accountUUID = crypto.randomUUID();
    const response = (await createAccount({
      accountId: accountUUID,
    })) as AccountObject;
    expect(response.account_id).toBe(accountUUID);
    accounts.push(response.account_id);

    const calendarResponse = (await createCalendar({
      accountId: accountUUID,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountUUID);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create an event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;
    const eventResponse = (await createEvent({
      calendarUid,
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
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
    })) as ErrorObject;
    expect(invalidCalendarResponse.error).toBe("Calendar not found");
  });

  test("Should not create an event with invalid start and end timestamps", async () => {
    const invalidStartAndEndResponse = (await createEvent({
      calendarUid,
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
      startTs,
      endTs: 0,
      scope: "single",
    })) as ErrorObject;
    expect(invalidEndAndStartResponse.error).toBe(
      "end_ts is required and must be positive"
    );

    const invalidEndAndStartResponse2 = (await updateEvent({
      eventUid,
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
    const response = await deleteEvent({ eventUid });
    expect(response.success).toBe(true);

    const getResponse = (await getEvent({ eventUid })) as ErrorObject;
    expect(getResponse.error).toBe("Event not found");
  });
});

describe("Recurring Event API", () => {
  const accounts: string[] = [];
  let calendarUid: string;
  let masterEventUid: string;

  afterAll(async () => {
    // deleting accounts will cascade to delete calendars and events
    for (const account of accounts) {
      const response = (await deleteAccount({
        accountId: account,
      })) as SuccessObject;
      expect(response.success).toBe(true);
    }
  });

  beforeAll(async () => {
    const accountUUID = crypto.randomUUID();
    const response = (await createAccount({
      accountId: accountUUID,
    })) as AccountObject;
    expect(response.account_id).toBe(accountUUID);
    accounts.push(response.account_id);

    const calendarResponse = (await createCalendar({
      accountId: accountUUID,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountUUID);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create a recurring event", async () => {
    const response = (await createEvent({
      calendarUid: calendarUid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
    })) as EventObject;
    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.event_uid).toBeDefined();
    expect(response.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(response.metadata).toEqual({ title: "Test Event" });

    const getResponse = (await getCalendarEvents({
      calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 30,
    })) as GenericPagedResponse<EventObject>;
    expect(getResponse.count).toBe(11);
    expect(getResponse.data).toHaveLength(11);
    expect(getResponse.data[0].calendar_uid).toBe(calendarUid);
    const masterEvent = getResponse.data.find(
      (e) => e.is_recurring_instance === false && e.recurrence !== null
    );
    expect(masterEvent).toBeDefined();
    expect(masterEvent?.event_uid).toBe(response.event_uid);
    expect(masterEvent?.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(masterEvent?.metadata).toEqual({ title: "Test Event" });
    masterEventUid = masterEvent?.event_uid as string;
  });

  test("Update single instance of recurring event", async () => {
    const response = (await updateEvent({
      eventUid: masterEventUid,
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
      calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 60,
    })) as GenericPagedResponse<EventObject>;
    expect(getResponse.count).toBe(11);
    expect(getResponse.data).toHaveLength(11);
    expect(getResponse.data[0].calendar_uid).toBe(calendarUid);
    const instanceEvent = getResponse.data.find(
      (e) => e.event_uid === masterEventUid
    );
    expect(instanceEvent).toBeDefined();
    expect(instanceEvent?.event_uid).toBe(response.event_uid);
    expect(instanceEvent?.recurrence).toEqual({ rule: "FREQ=DAILY;COUNT=10" });
    expect(instanceEvent?.metadata).toEqual({ title: "Updated Event" });

    const nonInstanceEvent = getResponse.data.find(
      (e) => e.event_uid !== masterEventUid
    );
    expect(nonInstanceEvent).toBeDefined();
    expect(nonInstanceEvent?.event_uid).not.toBe(masterEventUid);
    expect(nonInstanceEvent?.recurrence).toBeNull();
    expect(nonInstanceEvent?.metadata).toEqual({ title: "Test Event" });
  });

  test("Update entire series based on the master event of recurring event", async () => {
    const response = (await updateEvent({
      eventUid: masterEventUid,
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
      calendarUid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 86400 * 60,
    })) as GenericPagedResponse<EventObject>;
    expect(getResponse.count).toBe(11);
    expect(getResponse.data).toHaveLength(11);
    for (const event of getResponse.data) {
      expect(event.calendar_uid).toBe(calendarUid);
      expect(event.metadata).toEqual({ title: "Updated Event" });
    }
  });
});

describe("Recurring Event API - On-Demand Expansion", () => {
  const accounts: string[] = [];
  let calendarUid: string;
  let masterEventUid: string;

  beforeAll(async () => {
    const accountUUID = crypto.randomUUID();
    const response = (await createAccount({
      accountId: accountUUID,
    })) as AccountObject;
    expect(response.account_id).toBe(accountUUID);
    accounts.push(response.account_id);

    const calendarResponse = (await createCalendar({
      accountId: accountUUID,
    })) as CalendarObject;
    expect(calendarResponse.calendar_uid).toBeDefined();
    expect(calendarResponse.account_id).toBe(accountUUID);
    calendarUid = calendarResponse.calendar_uid;
  });

  test("Should create a recurring event with on-demand expansion", async () => {
    const response = (await createEvent({
      calendarUid: calendarUid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 3600 + 3600,
      metadata: { title: "Test Event" },
      recurrence: { rule: "FREQ=WEEKLY" },
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
      calendarUid,
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
      calendarUid,
      startTs,
      endTs,
    })) as GenericPagedResponse<EventObject>;
    expect(response.count).toBe(4);
    expect(response.data).toHaveLength(4);

    const firstEvent = response.data[0] as EventObject;
    expect(firstEvent.event_uid).toBe(masterEventUid);

    const updateResponse = (await updateEvent({
      eventUid: firstEvent.event_uid,
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
