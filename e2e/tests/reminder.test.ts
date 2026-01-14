import { beforeAll, describe, expect, test } from "vitest";
import { createEvent, getCalendarEvents } from "../helpers/event";
import { createCalendar } from "../helpers/calendar";
import {
  ErrorObject,
  EventObject,
  CalendarObject,
  ReminderObject,
  GenericPagedResponse,
} from "../helpers/types";
import {
  createReminder,
  deleteReminder,
  updateReminder,
  getEventReminders,
} from "../helpers/reminder";

const setup = async () => {
  let accountId: string = crypto.randomUUID();
  let calendar: CalendarObject;

  const calendarResponse = (await createCalendar({
    accountId: accountId,
  })) as CalendarObject;
  expect(calendarResponse.calendar_uid).toBeDefined();
  expect(calendarResponse.account_id).toBe(accountId);
  calendar = calendarResponse;

  return { accountId, calendar };
};

describe("Reminder API - single event", () => {
  let accountId: string;
  let calendar: CalendarObject;
  let event: EventObject;

  beforeAll(async () => {
    const { accountId: accountIdData, calendar: calendarData } = await setup();
    accountId = accountIdData;
    calendar = calendarData;

    const createdEvent = (await createEvent({
      accountId,
      calendarUid: calendar.calendar_uid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      endTs: Math.floor(Date.now() / 1000) + 7200,
      metadata: { title: "Test Event" },
    })) as EventObject;
    expect(createdEvent.event_uid).toBeDefined();
    event = createdEvent;
  });

  test("Should be able to create a reminder for a single event with no scope", async () => {
    const response = (await createReminder({
      metadata: {},
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -900, // 15 minutes before (negative)
    })) as any;

    expect(response.reminder).toBeDefined();
    expect(response.reminder.reminder_uid).toBeDefined();
    expect(response.reminder.account_id).toBe(accountId);
    expect(response.reminder.offset_seconds).toBe(-900);
    expect(response.reminder.metadata).toBeDefined();
    expect(response.reminder.is_delivered).toBe(false);
    expect(response.scope).toBe("single");
    expect(response.count).toBeGreaterThan(0);

    // Clean up
    await deleteReminder({
      reminderUid: response.reminder.reminder_uid,
      eventUid: event.event_uid,
      scope: "single",
    });
  });

  test("Should not be able to create a reminder with positive offset_seconds", async () => {
    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: 120, // Positive (invalid)
    })) as ErrorObject;

    expect(response.error).toBe(
      "offset_seconds must be negative (time before event)"
    );
  });

  test("Should be able to update a reminder successfully", async () => {
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -600, // 10 minutes before
      metadata: { title: "Test Event" },
      scope: "single",
    })) as any;

    expect(createResponse.reminder.reminder_uid).toBeDefined();
    expect(createResponse.reminder.account_id).toBe(accountId);
    expect(createResponse.reminder.offset_seconds).toBe(-600);
    expect(createResponse.reminder.metadata).toEqual({ title: "Test Event" });
    expect(createResponse.reminder.is_delivered).toBe(false);

    const updateResponse = (await updateReminder({
      reminderUid: createResponse.reminder.reminder_uid,
      eventUid: event.event_uid,
      offsetSeconds: -1800, // 30 minutes before
      metadata: { title: "Updated Test Event" },
      scope: "single",
    })) as any;

    expect(updateResponse.reminder.reminder_uid).toBeDefined();
    expect(updateResponse.reminder.account_id).toBe(accountId);
    expect(updateResponse.reminder.offset_seconds).toBe(-1800);
    expect(updateResponse.reminder.metadata).toEqual({
      title: "Updated Test Event",
    });
    expect(updateResponse.reminder.is_delivered).toBe(false);
    expect(updateResponse.scope).toBe("single");
    expect(updateResponse.count).toBe(1);

    // Fetch reminders for the event
    const reminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(reminders).toBeDefined();
    expect(reminders.length).toBeGreaterThan(0);
    expect(
      reminders.some(
        (r) => r.reminder_uid === updateResponse.reminder.reminder_uid
      )
    ).toBe(true);
    expect(reminders.some((r) => r.account_id === accountId)).toBe(true);
    expect(reminders.some((r) => r.offset_seconds === -1800)).toBe(true);
    expect(
      reminders.some((r) => r.metadata.title === "Updated Test Event")
    ).toBe(true);
    expect(reminders.some((r) => r.is_delivered === false)).toBe(true);

    // Clean up
    await deleteReminder({
      reminderUid: updateResponse.reminder.reminder_uid,
      eventUid: event.event_uid,
      scope: "single",
    });
  });

  test("Should be able to delete a reminder for a single event", async () => {
    // Create a reminder first
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -300, // 5 minutes before
      scope: "single",
    })) as any;

    expect(createResponse.reminder.reminder_uid).toBeDefined();

    // Get reminders before deletion
    let reminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(reminders.length).toBeGreaterThan(0);
    expect(
      reminders.some(
        (r) => r.reminder_uid === createResponse.reminder.reminder_uid
      )
    ).toBe(true);

    // Delete the reminder
    const deleteResponse = (await deleteReminder({
      eventUid: event.event_uid,
      reminderUid: createResponse.reminder.reminder_uid,
      scope: "single",
    })) as any;

    expect(deleteResponse.message || deleteResponse.success).toBeTruthy();
    expect(deleteResponse.scope).toBe("single");
    expect(deleteResponse.count).toBe(1);

    // Verify the reminder is deleted
    reminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(
      reminders.some(
        (r) => r.reminder_uid === createResponse.reminder.reminder_uid
      )
    ).toBe(false);
  });
});

describe("Reminder API - recurring event", () => {
  let accountId: string;
  let calendar: CalendarObject;
  let event: EventObject;

  beforeAll(async () => {
    const { accountId: accountIdData, calendar: calendarData } = await setup();
    accountId = accountIdData;
    calendar = calendarData;

    const createdEvent = (await createEvent({
      calendarUid: calendar.calendar_uid,
      startTs: Math.floor(Date.now() / 1000) + 3600,
      accountId,
      endTs: Math.floor(Date.now() / 1000) + 7200,
      metadata: { title: "Test Recurring Event" },
      recurrence: { rule: "FREQ=DAILY;COUNT=10" },
    })) as EventObject;
    expect(createdEvent.event_uid).toBeDefined();
    event = createdEvent;
  });

  test("Should not be able to create a reminder if a scope is not provided for a recurring event", async () => {
    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -600,
    })) as ErrorObject;

    expect(response.error).toBe(
      "Scope is required for recurring events (single or all)"
    );
  });

  test("Should be able to create a reminder for a recurring event with a single scope", async () => {
    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -900, // 15 minutes before
      scope: "single",
    })) as any;

    expect(response.reminder.reminder_uid).toBeDefined();
    expect(response.reminder.account_id).toBe(accountId);
    expect(response.reminder.offset_seconds).toBe(-900);
    expect(response.scope).toBe("single");
    expect(response.count).toBe(1);

    // Clean up
    const deleteResponse = (await deleteReminder({
      reminderUid: response.reminder.reminder_uid,
      eventUid: event.event_uid,
      scope: "single",
    })) as any;
    expect(deleteResponse.message || deleteResponse.success).toBeTruthy();
  });

  test("Should be able to create a reminder for a recurring event with 'all' scope", async () => {
    // Clean up any existing reminders first
    const existingReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    for (const reminder of existingReminders) {
      await deleteReminder({
        reminderUid: reminder.reminder_uid,
        eventUid: event.event_uid,
        scope: "all",
      });
    }

    // Create a reminder for the master event and all instances
    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -1200, // 20 minutes before
      scope: "all",
      metadata: { title: "Should be on all event instances" },
    })) as any;

    expect(response.reminder).toBeDefined();
    expect(response.reminder.account_id).toBe(accountId);
    expect(response.reminder.reminder_group_id).toBeDefined(); // All scope creates a group
    expect(response.scope).toBe("all");
    expect(response.count).toBeGreaterThan(1); // Master + instances

    // Verify master event has the reminder
    const masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(masterReminders.length).toBeGreaterThan(0);
    expect(
      masterReminders.some(
        (r) => r.reminder_group_id === response.reminder.reminder_group_id
      )
    ).toBe(true);

    // Get a future instance and verify it has the reminder
    const calendarEvents = (await getCalendarEvents({
      calendarUids: calendar.calendar_uid,
      startTs: event.start_ts + 86400,
      endTs: event.start_ts + 86400 * 2,
    })) as GenericPagedResponse<EventObject>;

    expect(calendarEvents.data.length).toBeGreaterThan(0);
    const instanceEvent = calendarEvents.data.find(
      (e) =>
        e.event_uid !== event.event_uid &&
        e.master_event_uid === event.event_uid
    );

    expect(instanceEvent).toBeDefined();
    expect(instanceEvent?.start_ts).toBeGreaterThan(event.start_ts);

    // Get reminders for the instance
    const instanceReminders = (await getEventReminders({
      eventUid: instanceEvent?.event_uid as string,
    })) as ReminderObject[];

    expect(instanceReminders.length).toBeGreaterThan(0);
    expect(
      instanceReminders.some(
        (r) => r.reminder_group_id === response.reminder.reminder_group_id
      )
    ).toBe(true);

    // Clean up - delete with 'all' scope
    await deleteReminder({
      reminderUid: masterReminders[0].reminder_uid,
      eventUid: event.event_uid,
      scope: "all",
    });
  });

  test("Should be able to delete a reminder for a recurring event with a single scope", async () => {
    // Create a reminder with 'all' scope first
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -600,
      scope: "all",
      metadata: { title: "Test reminder" },
    })) as any;

    const groupId = createResponse.reminder.reminder_group_id;
    expect(groupId).toBeDefined();

    // Get master reminders
    let masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(masterReminders.length).toBeGreaterThan(0);
    const masterReminder = masterReminders.find(
      (r) => r.reminder_group_id === groupId
    );
    expect(masterReminder).toBeDefined();

    // Delete with single scope (only affects this event)
    const deleteResponse = (await deleteReminder({
      reminderUid: masterReminder!.reminder_uid,
      eventUid: event.event_uid,
      scope: "single",
    })) as any;

    expect(deleteResponse.message || deleteResponse.success).toBeTruthy();
    expect(deleteResponse.scope).toBe("single");
    expect(deleteResponse.count).toBe(1);

    // Verify master event no longer has this reminder
    masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(
      masterReminders.some(
        (r) => r.reminder_uid === masterReminder!.reminder_uid
      )
    ).toBe(false);

    // Get a future instance and verify it still has the reminder
    const futureEvents = (await getCalendarEvents({
      calendarUids: calendar.calendar_uid,
      startTs: event.start_ts + 86400,
      endTs: event.start_ts + 86400 * 2,
    })) as GenericPagedResponse<EventObject>;

    const instanceEvent = futureEvents.data.find(
      (e) => e.event_uid !== event.event_uid
    );

    expect(instanceEvent).toBeDefined();

    const instanceReminders = (await getEventReminders({
      eventUid: instanceEvent!.event_uid,
    })) as ReminderObject[];

    expect(instanceReminders.some((r) => r.reminder_group_id === groupId)).toBe(
      true
    );

    // Clean up
    await deleteReminder({
      reminderUid: instanceReminders[0].reminder_uid,
      eventUid: instanceEvent!.event_uid,
      scope: "all",
    });
  });

  test("Should be able to delete a reminder for a recurring event with 'all' scope", async () => {
    // Clean existing reminders
    let masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    for (const reminder of masterReminders) {
      await deleteReminder({
        reminderUid: reminder.reminder_uid,
        eventUid: event.event_uid,
        scope: "all",
      });
    }

    // Create a reminder for all events
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -1800,
      scope: "all",
      metadata: { title: "Should be deleted from all" },
    })) as any;

    const groupId = createResponse.reminder.reminder_group_id;
    expect(groupId).toBeDefined();

    // Verify master has reminder
    masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(masterReminders.length).toBeGreaterThan(0);
    const masterReminder = masterReminders.find(
      (r) => r.reminder_group_id === groupId
    );
    expect(masterReminder).toBeDefined();

    // Delete with 'all' scope
    const deleteResponse = (await deleteReminder({
      reminderUid: masterReminder!.reminder_uid,
      scope: "all",
      eventUid: event.event_uid,
    })) as any;

    expect(deleteResponse.message || deleteResponse.success).toBeTruthy();
    expect(deleteResponse.scope).toBe("all");
    expect(deleteResponse.count).toBeGreaterThan(1); // Deleted from master + instances

    // Verify master has no reminders
    masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(masterReminders.length).toBe(0);

    // Verify future instance has no reminders with that group
    const futureEvents = (await getCalendarEvents({
      calendarUids: calendar.calendar_uid,
      startTs: event.start_ts + 86400,
      endTs: event.start_ts + 86400 * 2,
    })) as GenericPagedResponse<EventObject>;

    const instanceEvent = futureEvents.data.find(
      (e) => e.event_uid !== event.event_uid
    );

    expect(instanceEvent).toBeDefined();

    const instanceReminders = (await getEventReminders({
      eventUid: instanceEvent!.event_uid,
    })) as ReminderObject[];

    expect(
      instanceReminders.every((r) => r.reminder_group_id !== groupId)
    ).toBe(true);
  });

  test("Should be able to edit a reminder for a recurring event with a single scope", async () => {
    // Clean up first
    let masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    for (const reminder of masterReminders) {
      await deleteReminder({
        reminderUid: reminder.reminder_uid,
        eventUid: event.event_uid,
        scope: "all",
      });
    }

    // Create a reminder
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -600,
      scope: "single",
      metadata: { title: "Test Event" },
    })) as any;

    expect(createResponse.reminder.reminder_uid).toBeDefined();
    expect(createResponse.reminder.offset_seconds).toBe(-600);
    expect(createResponse.reminder.metadata).toEqual({ title: "Test Event" });

    // Update the reminder
    const updateResponse = (await updateReminder({
      reminderUid: createResponse.reminder.reminder_uid,
      eventUid: event.event_uid,
      offsetSeconds: -1200,
      metadata: { title: "Updated Test Event" },
      scope: "single",
    })) as any;

    expect(updateResponse.reminder.reminder_uid).toBeDefined();
    expect(updateResponse.reminder.offset_seconds).toBe(-1200);
    expect(updateResponse.reminder.metadata).toEqual({
      title: "Updated Test Event",
    });
    expect(updateResponse.scope).toBe("single");
    expect(updateResponse.count).toBe(1);

    // Verify the update
    const reminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    expect(reminders.length).toBeGreaterThan(0);
    expect(
      reminders.some(
        (r) => r.reminder_uid === updateResponse.reminder.reminder_uid
      )
    ).toBe(true);
    expect(reminders.some((r) => r.offset_seconds === -1200)).toBe(true);
    expect(
      reminders.some((r) => r.metadata.title === "Updated Test Event")
    ).toBe(true);

    // Clean up
    await deleteReminder({
      reminderUid: updateResponse.reminder.reminder_uid,
      eventUid: event.event_uid,
      scope: "single",
    });
  });

  test("Should be able to edit a reminder for a recurring event with 'all' scope", async () => {
    // Create a reminder with 'all' scope
    const createResponse = (await createReminder({
      eventUid: event.event_uid,
      accountId: accountId,
      offsetSeconds: -900,
      scope: "all",
      metadata: { title: "Original" },
    })) as any;

    const groupId = createResponse.reminder.reminder_group_id;
    expect(groupId).toBeDefined();

    // Get the master reminder
    let masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    const masterReminder = masterReminders.find(
      (r) => r.reminder_group_id === groupId
    );
    expect(masterReminder).toBeDefined();

    // Update with 'all' scope
    const updateResponse = (await updateReminder({
      reminderUid: masterReminder!.reminder_uid,
      eventUid: event.event_uid,
      offsetSeconds: -1800,
      metadata: { title: "Updated for all" },
      scope: "all",
    })) as any;

    expect(updateResponse.reminder.offset_seconds).toBe(-1800);
    expect(updateResponse.scope).toBe("all");
    expect(updateResponse.count).toBeGreaterThan(1); // Updated master + future instances

    // Verify master has updated reminder
    masterReminders = (await getEventReminders({
      eventUid: event.event_uid,
    })) as ReminderObject[];

    const updatedMasterReminder = masterReminders.find(
      (r) => r.reminder_group_id === groupId
    );
    expect(updatedMasterReminder).toBeDefined();
    expect(updatedMasterReminder!.offset_seconds).toBe(-1800);
    expect(updatedMasterReminder!.metadata.title).toBe("Updated for all");

    // Verify future instance has updated reminder
    const futureEvents = (await getCalendarEvents({
      calendarUids: calendar.calendar_uid,
      startTs: event.start_ts + 86400,
      endTs: event.start_ts + 86400 * 2,
    })) as GenericPagedResponse<EventObject>;

    const instanceEvent = futureEvents.data.find(
      (e) => e.event_uid !== event.event_uid
    );
    expect(instanceEvent).toBeDefined();

    const instanceReminders = (await getEventReminders({
      eventUid: instanceEvent!.event_uid,
    })) as ReminderObject[];

    const instanceReminder = instanceReminders.find(
      (r) => r.reminder_group_id === groupId
    );
    expect(instanceReminder).toBeDefined();
    expect(instanceReminder!.offset_seconds).toBe(-1800);
    expect(instanceReminder!.metadata.title).toBe("Updated for all");

    // Clean up
    await deleteReminder({
      reminderUid: masterReminder!.reminder_uid,
      eventUid: event.event_uid,
      scope: "all",
    });
  });
});
