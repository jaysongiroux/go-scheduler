import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  createAttendee,
  deleteAttendee,
  getAttendee,
  getAttendeeEvents,
  getEventAttendees,
  transferOwnership,
  updateAttendeeRSVP,
} from "../helpers/attendee";
import { createCalendar, deleteCalendar } from "../helpers/calendar";
import { deleteAccount } from "../helpers/account";
import { createEvent, deleteEvent, getEvent } from "../helpers/event";
import { createReminder, getEventReminders } from "../helpers/reminder";
import { sleep } from "../helpers/general";
import { AttendeeObject, ErrorObject } from "../helpers/types";

describe("Attendee Management", () => {
  let accountId1: string;
  let accountId2: string;
  let accountId3: string;
  let calendarUid1: string;
  let calendarUid2: string;

  beforeAll(async () => {
    // Create test accounts
    accountId1 = `test-account-${Date.now()}-1`;
    accountId2 = `test-account-${Date.now()}-2`;
    accountId3 = `test-account-${Date.now()}-3`;

    // Create calendars for testing
    const cal1 = await createCalendar({ accountId: accountId1 });
    const cal2 = await createCalendar({ accountId: accountId2 });
    if ("error" in cal1 || "error" in cal2) {
      throw new Error("Failed to create test calendars");
    }
    calendarUid1 = cal1.calendar_uid;
    calendarUid2 = cal2.calendar_uid;
  });

  afterAll(async () => {
    // Cleanup
    await deleteCalendar({ calendarUid: calendarUid1 });
    await deleteCalendar({ calendarUid: calendarUid2 });
    await deleteAccount({ accountId: accountId1 });
    await deleteAccount({ accountId: accountId2 });
    await deleteAccount({ accountId: accountId3 });
  });

  describe("Single Event Attendees", () => {
    test("should auto-create organizer when creating an event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Test Event with Auto-Organizer" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Check that organizer was auto-created
      const attendees = await getEventAttendees({ eventUid: event.event_uid });
      if ("error" in attendees) {
        throw new Error("Failed to get attendees");
      }

      expect(attendees).toHaveLength(1);
      expect(attendees[0].account_id).toBe(accountId1);
      expect(attendees[0].role).toBe("organizer");
      expect(attendees[0].rsvp_status).toBe("accepted");

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should add an attendee to a single event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Test Event" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Add attendee
      const result = await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
        role: "attendee",
        metadata: { note: "Test attendee" },
      });

      if ("error" in result) {
        throw new Error(`Failed to add attendee: ${result.error}`);
      }

      expect("attendee" in result).toBe(true);
      if ("attendee" in result) {
        expect(result.attendee.account_id).toBe(accountId2);
        expect(result.attendee.role).toBe("attendee");
        expect(result.attendee.rsvp_status).toBe("pending");
        expect(result.scope).toBe("single");
        expect(result.count).toBe(1);
      }

      // Verify attendee was added
      const attendee = await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      if ("error" in attendee) {
        throw new Error("Failed to get attendee");
      }

      expect(attendee.account_id).toBe(accountId2);
      expect(attendee.role).toBe("attendee");

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should update RSVP status for single event", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "RSVP Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Update RSVP
      const result = await updateAttendeeRSVP({
        eventUid: event.event_uid,
        accountId: accountId2,
        rsvpStatus: "accepted",
      });

      if ("error" in result) {
        throw new Error(`Failed to update RSVP: ${result.error}`);
      }

      expect(result.count).toBe(1);

      // Verify RSVP was updated
      const attendee = await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      if ("error" in attendee) {
        throw new Error("Failed to get attendee");
      }

      expect(attendee.rsvp_status).toBe("accepted");

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should not allow RSVP update after event has ended", async () => {
      const now = Math.floor(Date.now() / 1000);
      // Create event that has already ended
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now - 7200, // 2 hours ago
        endTs: now - 3600, // 1 hour ago
        metadata: { title: "Past Event" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Try to update RSVP
      const result = await updateAttendeeRSVP({
        eventUid: event.event_uid,
        accountId: accountId2,
        rsvpStatus: "accepted",
      });

      expect("error" in result).toBe(true);
      if ("error" in result) {
        expect(result.error.toLowerCase()).toContain("ended");
      }

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should not allow removing last organizer", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Organizer Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Try to remove the only organizer
      const result = await deleteAttendee({
        eventUid: event.event_uid,
        accountId: accountId1,
      });

      expect("error" in result).toBe(true);
      if ("error" in result) {
        expect(result.error.toLowerCase()).toContain("last organizer");
      }

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should soft-delete attendee and their reminders", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Delete Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Add attendee
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Create reminder for attendee
      await createReminder({
        eventUid: event.event_uid,
        accountId: accountId2,
        offsetSeconds: -900, // 15 minutes before
      });

      // Verify reminder exists
      const remindersBefore = await getEventReminders({
        eventUid: event.event_uid,
        accountId: accountId2,
      });
      if (!("error" in remindersBefore)) {
        expect(remindersBefore.length).toBeGreaterThan(0);
      }

      // Delete attendee
      const result = await deleteAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      if ("error" in result) {
        throw new Error(`Failed to delete attendee: ${result.error}`);
      }

      expect(result.count).toBe(1);
      expect(result.reminders_deleted).toBeGreaterThan(0);

      // Verify attendee is gone
      const attendeeAfter = await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      expect("error" in attendeeAfter).toBe(true);

      // Verify reminders are gone
      const remindersAfter = await getEventReminders({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      if (!("error" in remindersAfter)) {
        expect(remindersAfter).toHaveLength(0);
      }

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should only allow attendees to create reminders", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Reminder Permission Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Try to create reminder as non-attendee
      const result = await createReminder({
        eventUid: event.event_uid,
        accountId: accountId3, // Not an attendee
        offsetSeconds: -900,
      });

      expect("error" in result).toBe(true);
      if ("error" in result) {
        expect(result.error.toLowerCase()).toContain("attendee");
      }

      // Now add as attendee and try again
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId3,
      });

      const result2 = await createReminder({
        eventUid: event.event_uid,
        accountId: accountId3,
        offsetSeconds: -900,
      });

      expect("error" in result2).toBe(false);

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should re-invite previously removed attendee", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Re-invite Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Add attendee
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Remove attendee
      await deleteAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Re-invite (should un-archive)
      const result = await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      expect("error" in result).toBe(false);

      // Verify attendee is back
      const attendee = (await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      })) as AttendeeObject;

      expect("error" in attendee).toBe(false);
      if ("error" in attendee) {
        expect(attendee.archived).toBe(false);
      }

      await deleteEvent({ eventUid: event.event_uid });
    });
  });

  describe("Recurring Event Attendees", () => {
    test("should add attendee to all events in series", async () => {
      const now = Math.floor(Date.now() / 1000);
      const startDate = new Date((now + 86400) * 1000); // tomorrow
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: Math.floor(startDate.getTime() / 1000),
        endTs: Math.floor(startDate.getTime() / 1000) + 3600,
        recurrence: {
          rule: "FREQ=DAILY;COUNT=5",
        },
        metadata: { title: "Recurring Series" },
      });

      if ("error" in event) {
        throw new Error("Failed to create recurring event");
      }

      // Wait for instances to be generated
      await sleep(2000);

      // Add attendee to all events
      const result = await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
        scope: "all",
      });

      if ("error" in result) {
        throw new Error(`Failed to add attendee: ${result.error}`);
      }

      expect("attendee_group_id" in result).toBe(true);
      expect(result.count).toBeGreaterThan(1);
      expect(result.scope).toBe("all");

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should update RSVP for all events in series", async () => {
      const now = Math.floor(Date.now() / 1000);
      const startDate = new Date((now + 86400) * 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: Math.floor(startDate.getTime() / 1000),
        endTs: Math.floor(startDate.getTime() / 1000) + 3600,
        recurrence: {
          rule: "FREQ=DAILY;COUNT=3",
        },
        metadata: { title: "RSVP Series Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create recurring event");
      }

      await sleep(2000);

      // Add attendee to all
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
        scope: "all",
      });

      // Update RSVP for all
      const result = await updateAttendeeRSVP({
        eventUid: event.event_uid,
        accountId: accountId2,
        rsvpStatus: "accepted",
        scope: "all",
      });

      if ("error" in result) {
        throw new Error(`Failed to update RSVP: ${result.error}`);
      }

      expect(result.count).toBeGreaterThan(1);
      expect(result.scope).toBe("all");

      await deleteEvent({ eventUid: event.event_uid });
    });

    test("should remove attendee from all future instances", async () => {
      const now = Math.floor(Date.now() / 1000);
      const startDate = new Date((now + 86400) * 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: Math.floor(startDate.getTime() / 1000),
        endTs: Math.floor(startDate.getTime() / 1000) + 3600,
        recurrence: {
          rule: "FREQ=DAILY;COUNT=5",
        },
        metadata: { title: "Delete Series Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create recurring event");
      }

      await sleep(2000);

      // Add attendee to all
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
        scope: "all",
      });

      // Delete attendee from all
      const result = await deleteAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
        scope: "all",
      });

      if ("error" in result) {
        throw new Error(`Failed to delete attendee: ${result.error}`);
      }

      expect(result.count).toBeGreaterThan(1);
      expect(result.scope).toBe("all");

      await deleteEvent({ eventUid: event.event_uid });
    });
  });

  describe("Ownership Transfer", () => {
    test("should transfer event ownership to another user", async () => {
      const now = Math.floor(Date.now() / 1000);
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Transfer Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Transfer ownership
      const result = await transferOwnership({
        eventUid: event.event_uid,
        newOrganizerAccountId: accountId2,
        newOrganizerCalendarUid: calendarUid2,
      });

      if ("error" in result) {
        throw new Error(`Failed to transfer ownership: ${result.error}`);
      }

      expect(result.new_organizer.account_id).toBe(accountId2);
      expect(result.new_organizer.calendar_uid).toBe(calendarUid2);

      // Verify event ownership changed
      const updatedEvent = await getEvent({ eventUid: event.event_uid });
      if (!("error" in updatedEvent)) {
        expect(updatedEvent.account_id).toBe(accountId2);
        expect(updatedEvent.calendar_uid).toBe(calendarUid2);
      }

      // Verify old organizer is now attendee
      const oldOrganizer = await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId1,
      });
      if (!("error" in oldOrganizer)) {
        expect(oldOrganizer.role).toBe("attendee");
      }

      // Verify new organizer is organizer
      const newOrganizer = await getAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });
      if (!("error" in newOrganizer)) {
        expect(newOrganizer.role).toBe("organizer");
      }

      await deleteEvent({ eventUid: event.event_uid });
    });
  });

  describe("Get Invited Events", () => {
    test("should retrieve events where user is invited", async () => {
      const now = Math.floor(Date.now() / 1000);

      // Create event as account1
      const event = await createEvent({
        accountId: accountId1,
        calendarUid: calendarUid1,
        startTs: now + 3600,
        endTs: now + 7200,
        metadata: { title: "Invitation Test" },
      });

      if ("error" in event) {
        throw new Error("Failed to create event");
      }

      // Invite account2
      await createAttendee({
        eventUid: event.event_uid,
        accountId: accountId2,
      });

      // Get invited events for account2
      const result = await getAttendeeEvents({
        accountId: accountId2,
        startTs: now,
        endTs: now + 86400,
        role: "attendee", // Exclude events they organize
      });

      if ("error" in result) {
        throw new Error(`Failed to get invited events: ${result.error}`);
      }

      expect(result.length).toBeGreaterThan(0);
      const foundEvent = result.find(
        (e) => e.event.event_uid === event.event_uid
      );
      expect(foundEvent).toBeDefined();
      if (foundEvent) {
        expect(foundEvent.attendee.role).toBe("attendee");
      }

      await deleteEvent({ eventUid: event.event_uid });
    });
  });
});
