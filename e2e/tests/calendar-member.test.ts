import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  CalendarObject,
  CalendarMemberObject,
  ErrorObject,
  SuccessObject,
  GenericPagedResponse,
} from "../helpers/types";
import {
  createCalendar,
  deleteCalendar,
  getCalendar,
  getUserCalendars,
} from "../helpers/calendar";
import {
  inviteCalendarMembers,
  getCalendarMembers,
  updateCalendarMember,
  removeCalendarMember,
} from "../helpers/calendar-member";
import {
  createWebhook,
  deleteWebhook,
  getWebhookDeliveries,
} from "../helpers/webhook";
import { startWebhookServer } from "../helpers/webhook-server";
import { WebhookObject, WebhookDeliveryObject } from "../helpers/types";
import { sleep } from "../helpers/general";

describe("Calendar Member API", () => {
  let calendarUid: string;
  let ownerAccountId: string = crypto.randomUUID();
  let member1AccountId: string = crypto.randomUUID();
  let member2AccountId: string = crypto.randomUUID();
  let webhookUid: string;
  let webhookUrl: string;
  let webhookServer: { getLastEvent: () => any; close: () => void };

  beforeAll(() => {
    // Start webhook server
    webhookServer = startWebhookServer(3335);
  });

  afterAll(async () => {
    webhookServer.close();
  });

  test("Should create a calendar", async () => {
    const response = (await createCalendar({
      accountId: ownerAccountId,
    })) as CalendarObject;
    expect(response.calendar_uid).toBeDefined();
    expect(response.account_id).toBe(ownerAccountId);
    calendarUid = response.calendar_uid;
  });

  test("Should create webhook for member events", async () => {
    const response = (await createWebhook({
      isActive: true,
      url: "http://localhost:3335/webhook",
      eventTypes: [
        "member.invited",
        "member.accepted",
        "member.rejected",
        "member.removed",
      ],
    })) as WebhookObject;
    expect(response.webhook_uid).toBeDefined();
    webhookUid = response.webhook_uid;
  });

  test("Should invite members to calendar", async () => {
    const response = (await inviteCalendarMembers({
      calendarUid: calendarUid,
      accountIds: [member1AccountId, member2AccountId],
      accountId: ownerAccountId,
      role: "write",
    })) as { invited_count: number; members: CalendarMemberObject[] };

    expect(response.invited_count).toBe(2);
    expect(response.members).toHaveLength(2);
    expect(response.members[0].status).toBe("pending");
    expect(response.members[0].role).toBe("write");
    expect(response.members[0].invited_by).toBe(ownerAccountId);
    await sleep(3000); // Wait for webhook delivery

    const deliveries = (await getWebhookDeliveries({
      webhookUid: webhookUid,
      limit: 100,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const inviteDeliveries = deliveries.data.filter(
      (d) => d.event_type === "member.invited"
    );
    expect(inviteDeliveries.length).toBeGreaterThanOrEqual(2);
  });

  test("Should get calendar members", async () => {
    const response = (await getCalendarMembers({
      calendarUid: calendarUid,
      accountId: ownerAccountId,
    })) as CalendarMemberObject[];

    expect(response).toHaveLength(2);
    expect(response.some((m) => m.account_id === member1AccountId)).toBe(true);
    expect(response.some((m) => m.account_id === member2AccountId)).toBe(true);
  });

  test("Should include members in get calendar response", async () => {
    const response = (await getCalendar({
      calendarUid: calendarUid,
    })) as CalendarObject;

    expect(response.calendar_uid).toBe(calendarUid);
    expect(response.members).toBeDefined();
    expect(response.members).toHaveLength(2);
  });

  test("Should accept invitation", async () => {
    const response = (await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: member1AccountId,
      requestingAccountId: member1AccountId,
      status: "confirmed",
    })) as CalendarMemberObject;

    expect(response.status).toBe("confirmed");
    expect(response.account_id).toBe(member1AccountId);
  });

  test("Should receive member.accepted webhook", async () => {
    await sleep(2000); // Wait for webhook delivery

    const deliveries = (await getWebhookDeliveries({
      webhookUid: webhookUid,
      limit: 100,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const acceptDeliveries = deliveries.data.filter(
      (d) => d.event_type === "member.accepted"
    );
    expect(acceptDeliveries.length).toBeGreaterThanOrEqual(1);
  });

  test("Should reject invitation (delete member)", async () => {
    const response = (await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: member2AccountId,
      requestingAccountId: member2AccountId,
      status: "pending",
    })) as SuccessObject;

    expect(response.success).toBe(true);
  });

  test("Should receive member.rejected webhook", async () => {
    await sleep(2000); // Wait for webhook delivery

    const deliveries = (await getWebhookDeliveries({
      webhookUid: webhookUid,
      limit: 100,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const rejectDeliveries = deliveries.data.filter(
      (d) => d.event_type === "member.rejected"
    );
    expect(rejectDeliveries.length).toBeGreaterThanOrEqual(1);
  });

  test("Should show calendar in member's calendar list", async () => {
    const response = (await getUserCalendars({
      accountId: member1AccountId,
    })) as { data: CalendarObject[] };

    expect(response.data).toHaveLength(1);
    expect(response.data[0].calendar_uid).toBe(calendarUid);
    expect(response.data[0].is_owner).toBe(false);
    expect(response.data[0].members).toBeDefined();
  });

  test("Should show calendar in owner's calendar list", async () => {
    const response = (await getUserCalendars({
      accountId: ownerAccountId,
    })) as { data: CalendarObject[] };

    expect(response.data).toHaveLength(1);
    expect(response.data[0].calendar_uid).toBe(calendarUid);
    expect(response.data[0].is_owner).toBe(true);
  });

  test("Should invite another member with read role", async () => {
    const newMember = crypto.randomUUID();
    const response = (await inviteCalendarMembers({
      calendarUid: calendarUid,
      accountId: ownerAccountId,
      accountIds: [newMember],
      role: "read",
    })) as { invited_count: number; members: CalendarMemberObject[] };

    expect(response.invited_count).toBe(1);
    expect(response.members[0].role).toBe("read");

    // Accept the invitation
    await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: newMember,
      requestingAccountId: newMember,
      status: "confirmed",
    });
  });

  test("Should update member role", async () => {
    const response = (await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: member1AccountId,
      requestingAccountId: ownerAccountId,
      role: "read",
    })) as CalendarMemberObject;

    expect(response.role).toBe("read");
  });

  test("Non-owner should not be able to change roles", async () => {
    const response = (await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: member1AccountId,
      requestingAccountId: member1AccountId,
      role: "write",
    })) as ErrorObject;

    expect(response.error).toBeDefined();
  });

  test("Owner should remove member", async () => {
    const response = (await removeCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: member1AccountId,
      requestingAccountId: ownerAccountId,
    })) as SuccessObject;

    expect(response.success).toBe(true);
  });

  test("Should receive member.removed webhook", async () => {
    await sleep(2000); // Wait for webhook delivery

    const deliveries = (await getWebhookDeliveries({
      webhookUid: webhookUid,
      limit: 100,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const removeDeliveries = deliveries.data.filter(
      (d) => d.event_type === "member.removed"
    );
    expect(removeDeliveries.length).toBeGreaterThanOrEqual(1);
  });

  test("Member should be able to remove themselves", async () => {
    // First invite and accept a new member
    const selfRemoveMember = crypto.randomUUID();
    await inviteCalendarMembers({
      calendarUid: calendarUid,
      accountId: ownerAccountId,
      accountIds: [selfRemoveMember],
      role: "write",
    });

    await updateCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: selfRemoveMember,
      requestingAccountId: selfRemoveMember,
      status: "confirmed",
    });

    // Now remove themselves
    const response = (await removeCalendarMember({
      calendarUid: calendarUid,
      memberAccountId: selfRemoveMember,
      requestingAccountId: selfRemoveMember,
    })) as SuccessObject;

    expect(response.success).toBe(true);
  });

  test("Non-owner/non-self should not be able to view members", async () => {
    const randomAccount = crypto.randomUUID();
    const response = (await getCalendarMembers({
      calendarUid: calendarUid,
      accountId: randomAccount,
    })) as ErrorObject;

    expect(response.error).toBeDefined();
  });

  test("Should clean up calendar", async () => {
    await deleteCalendar({ calendarUid: calendarUid });
  });

  test("Should clean up webhook", async () => {
    await deleteWebhook({ webhookUid: webhookUid });
  });
});
