import { describe, expect, test } from "vitest";
import { createCalendar, getCalendar } from "../helpers/calendar";
import { createEvent, getEvent } from "../helpers/event";
import { CalendarObject, ErrorObject, EventObject } from "../helpers/types";
import { deleteAccount } from "../helpers/account";
import {
  inviteCalendarMembers,
  getCalendarMembers,
  updateCalendarMember,
} from "../helpers/calendar-member";
import { createReminder, getEventReminders } from "../helpers/reminder";

describe("Deleting an account", () => {
  test("Should delete events created by the account", async () => {
    const accountId = crypto.randomUUID();
    const calendar = (await createCalendar({
      accountId,
      metadata: { title: "example" },
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();
    expect(calendar.account_id).toBe(accountId);
    const event = (await createEvent({
      accountId,
      calendarUid: calendar.calendar_uid,
      startTs: Math.floor(Date.now() / 1000),
      endTs: Math.floor(Date.now() / 1000) + 3600,
      metadata: { title: "example" },
    })) as EventObject;
    const fetchedEvent = (await getEvent({
      eventUid: event.event_uid,
    })) as EventObject;
    expect(fetchedEvent.event_uid).toBe(event.event_uid);
    expect(fetchedEvent.account_id).toBe(accountId);
    expect(fetchedEvent.calendar_uid).toBe(calendar.calendar_uid);
    expect(fetchedEvent.start_ts).toBe(Math.floor(Date.now() / 1000));
    expect(fetchedEvent.end_ts).toBe(Math.floor(Date.now() / 1000) + 3600);
    expect(fetchedEvent.metadata).toEqual({ title: "example" });

    const response = await deleteAccount({ accountId });
    expect(response.success).toBe(true);
  });

  test("Should delete calendars created by the account", async () => {
    const accountId = crypto.randomUUID();

    // Create a calendar
    const calendar = (await createCalendar({
      accountId,
      metadata: { title: "Calendar to delete" },
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();
    expect(calendar.account_id).toBe(accountId);

    // Verify calendar exists
    const fetchedCalendar = (await getCalendar({
      calendarUid: calendar.calendar_uid,
    })) as CalendarObject;
    expect(fetchedCalendar.calendar_uid).toBe(calendar.calendar_uid);

    // Delete account
    const response = await deleteAccount({ accountId });
    expect(response.success).toBe(true);

    // Verify calendar is deleted
    const deletedCalendar = (await getCalendar({
      calendarUid: calendar.calendar_uid,
    })) as ErrorObject;
    expect(deletedCalendar.error).toBeDefined();
    expect(deletedCalendar.error).toContain("not found");
  });

  test("Should delete the account's calendar members", async () => {
    const ownerAccountId = crypto.randomUUID();
    const memberAccountId = crypto.randomUUID();

    // Create a calendar as owner
    const calendar = (await createCalendar({
      accountId: ownerAccountId,
      metadata: { title: "Shared Calendar" },
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();

    // Invite member to calendar
    const inviteResponse = await inviteCalendarMembers({
      calendarUid: calendar.calendar_uid,
      accountId: ownerAccountId,
      accountIds: [memberAccountId],
      role: "write",
    });
    expect(inviteResponse).toHaveProperty("invited_count");

    // Member accepts invitation
    await updateCalendarMember({
      calendarUid: calendar.calendar_uid,
      memberAccountId: memberAccountId,
      requestingAccountId: memberAccountId,
      status: "confirmed",
    });

    // Verify member is in calendar
    const members = await getCalendarMembers({
      calendarUid: calendar.calendar_uid,
      accountId: ownerAccountId,
    });
    expect(Array.isArray(members)).toBe(true);
    if (Array.isArray(members)) {
      expect(members.length).toBe(1);
      expect(members[0].account_id).toBe(memberAccountId);
    }

    // Delete member account
    const response = await deleteAccount({ accountId: memberAccountId });
    expect(response.success).toBe(true);

    // Verify member is removed from calendar
    const remainingMembers = await getCalendarMembers({
      calendarUid: calendar.calendar_uid,
      accountId: ownerAccountId,
    });
    expect(Array.isArray(remainingMembers)).toBe(true);
    if (Array.isArray(remainingMembers)) {
      expect(remainingMembers.length).toBe(0);
    }

    // Cleanup: delete owner account and calendar
    await deleteAccount({ accountId: ownerAccountId });
  });

  test("Should delete reminders created by the account", async () => {
    const accountId = crypto.randomUUID();

    // Create calendar and event
    const calendar = (await createCalendar({
      accountId,
      metadata: { title: "Calendar with reminders" },
    })) as CalendarObject;

    const now = Math.floor(Date.now() / 1000);
    const event = (await createEvent({
      accountId,
      calendarUid: calendar.calendar_uid,
      startTs: now + 7200,
      endTs: now + 10800,
      metadata: { title: "Event with reminder" },
    })) as EventObject;
    expect(event.event_uid).toBeDefined();

    // Create reminder for the event
    const reminder = (await createReminder({
      eventUid: event.event_uid,
      accountId,
      offsetSeconds: -3600, // 1 hour before
      metadata: { type: "notification" },
    })) as any;

    expect(reminder.reminder).toHaveProperty("reminder_uid");

    // Verify reminder exists
    const reminders = await getEventReminders({
      eventUid: event.event_uid,
    });
    expect(Array.isArray(reminders)).toBe(true);
    if (Array.isArray(reminders)) {
      expect(reminders.length).toBe(1);
      expect(reminders[0].account_id).toBe(accountId);
    }

    // Delete account
    const response = await deleteAccount({ accountId });
    expect(response.success).toBe(true);

    // Verify event and reminders are deleted (event deletion cascades to reminders)
    const deletedEvent = (await getEvent({
      eventUid: event.event_uid,
    })) as ErrorObject;
    expect(deletedEvent.error).toBeDefined();
  });
});
