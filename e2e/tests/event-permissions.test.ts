import { describe, expect, test, beforeAll } from "vitest";
import { createEvent, deleteEvent, updateEvent } from "../helpers/event";
import { createCalendar, deleteCalendar } from "../helpers/calendar";
import {
  inviteCalendarMembers,
  updateCalendarMember,
} from "../helpers/calendar-member";
import { ErrorObject, EventObject, CalendarObject } from "../helpers/types";

describe("Event Permissions with Shared Calendars", () => {
  let ownerAccountId: string = crypto.randomUUID();
  let writeMemberAccountId: string = crypto.randomUUID();
  let readMemberAccountId: string = crypto.randomUUID();
  let nonMemberAccountId: string = crypto.randomUUID();
  let calendarUid: string;
  let eventUid: string;

  beforeAll(async () => {
    // Create calendar as owner
    const calendarResponse = (await createCalendar({
      accountId: ownerAccountId,
    })) as CalendarObject;
    calendarUid = calendarResponse.calendar_uid;

    // Invite members with different roles
    await inviteCalendarMembers({
      calendarUid: calendarUid,
      accountId: ownerAccountId,
      accountIds: [writeMemberAccountId, readMemberAccountId],
      role: "write",
    });

    // Accept invitations
    await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: writeMemberAccountId,
      requestingAccountId: writeMemberAccountId,
      status: "confirmed",
    });

    await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: readMemberAccountId,
      requestingAccountId: readMemberAccountId,
      status: "confirmed",
    });

    // Change read member to read-only
    await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: readMemberAccountId,
      requestingAccountId: ownerAccountId,
      role: "read",
    });
  });

  test("Owner should be able to create event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 3600;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId: ownerAccountId,
      startTs,
      endTs,
      metadata: { title: "Owner Event" },
    })) as EventObject;

    expect(response.event_uid).toBeDefined();
    expect(response.calendar_uid).toBe(calendarUid);
    eventUid = response.event_uid;
  });

  test("Write member should be able to create event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 7200;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId: writeMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Write Member Event" },
    })) as EventObject;

    expect(response.event_uid).toBeDefined();
    expect(response.calendar_uid).toBe(calendarUid);
  });

  test("Read-only member should NOT be able to create event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 10800;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId: readMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Read Member Event" },
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Non-member should NOT be able to create event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 14400;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId: nonMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Non-member Event" },
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Write member should be able to update event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 18000;
    const endTs = startTs + 3600;

    const response = (await updateEvent({
      eventUid,
      accountId: writeMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Updated by Write Member" },
    })) as EventObject;

    expect(response.event_uid).toBe(eventUid);
    expect(response.metadata).toEqual({ title: "Updated by Write Member" });
  });

  test("Read-only member should NOT be able to update event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 21600;
    const endTs = startTs + 3600;

    const response = (await updateEvent({
      eventUid,
      accountId: readMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Updated by Read Member" },
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Non-member should NOT be able to update event", async () => {
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 25200;
    const endTs = startTs + 3600;

    const response = (await updateEvent({
      eventUid,
      accountId: nonMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Updated by Non-member" },
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Read-only member should NOT be able to delete event", async () => {
    const response = (await deleteEvent({
      eventUid,
      accountId: readMemberAccountId,
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Non-member should NOT be able to delete event", async () => {
    const response = (await deleteEvent({
      eventUid,
      accountId: nonMemberAccountId,
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  test("Write member should be able to delete event", async () => {
    // Create a new event to delete
    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 28800;
    const endTs = startTs + 3600;

    const createResponse = (await createEvent({
      calendarUid,
      accountId: writeMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Event to Delete" },
    })) as EventObject;

    const deleteResponse = (await deleteEvent({
      eventUid: createResponse.event_uid,
      accountId: writeMemberAccountId,
    })) as { success: boolean };

    expect(deleteResponse.success).toBe(true);
  });

  test("Owner should be able to delete event", async () => {
    const response = (await deleteEvent({
      eventUid,
      accountId: ownerAccountId,
    })) as { success: boolean };

    expect(response.success).toBe(true);
  });

  test("Pending member should NOT be able to create event", async () => {
    const pendingMemberAccountId = crypto.randomUUID();

    // Invite but don't accept
    await inviteCalendarMembers({
      calendarUid: calendarUid,
      accountId: ownerAccountId,
      accountIds: [pendingMemberAccountId],
      role: "write",
    });

    const now = Math.floor(Date.now() / 1000);
    const startTs = now + 32400;
    const endTs = startTs + 3600;

    const response = (await createEvent({
      calendarUid,
      accountId: pendingMemberAccountId,
      startTs,
      endTs,
      metadata: { title: "Pending Member Event" },
    })) as ErrorObject;

    expect(response.error).toBeDefined();
    expect(response.error).toContain("permission");
  });

  // Cleanup
  test("Should clean up calendar", async () => {
    await deleteCalendar({ calendarUid: calendarUid });
  });
});
