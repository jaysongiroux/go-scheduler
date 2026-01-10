import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  createWebhook,
  deleteWebhook,
  getWebhook,
  getWebhookDeliveries,
  updateWebhook,
} from "../helpers/webhook";
import {
  AccountObject,
  CalendarObject,
  EventObject,
  GenericPagedResponse,
  ReminderObject,
  SuccessObject,
  WebhookDeliveryObject,
  WebhookObject,
} from "../helpers/types";
import { startWebhookServer } from "../helpers/webhook-server";
import { createAccount, deleteAccount } from "../helpers/account";
import { wait } from "../helpers/general";
import { createEvent, toggleCancelledStatusEvent } from "../helpers/event";
import { createCalendar } from "../helpers/calendar";
import {
  createReminder,
  deleteReminder,
  updateReminder,
} from "../helpers/reminder";

describe("Webhook API", () => {
  let webhookUid: string;
  let secret: string;

  afterAll(async () => {
    await deleteWebhook({ webhookUid });
  });

  test("Should create a webhook", async () => {
    const webhook = (await createWebhook({
      isActive: true,
      url: "https://example.com/webhook",
      eventTypes: ["event.created"],
    })) as WebhookObject;
    webhookUid = webhook.webhook_uid;
    expect(webhook.webhook_uid).toBeDefined();
    expect(webhook.is_active).toBe(true);
    secret = webhook.secret;
  });

  test("Should get a webhook", async () => {
    const webhook = (await getWebhook({ webhookUid })) as WebhookObject;
    expect(webhook.webhook_uid).toBe(webhookUid);
    expect(webhook.is_active).toBe(true);
  });

  test("Should update webhook", async () => {
    const webhook = (await updateWebhook({
      webhookUid,
      url: "http://localhost:3000/webhook",
      eventTypes: ["event.updated"],
      isActive: false,
      retryCount: 5,
      timeoutSeconds: 60,
    })) as WebhookObject;
    expect(webhook.webhook_uid).toBe(webhookUid);
    expect(webhook.is_active).toBe(false);
    expect(webhook.url).toBe("http://localhost:3000/webhook");
    expect(webhook.event_types).toEqual(["event.updated"]);
    expect(webhook.retry_count).toBe(5);
    expect(webhook.timeout_seconds).toBe(60);
  });

  test("Should not update secret", async () => {
    const webhook = (await updateWebhook({
      webhookUid,
      url: "http://localhost:3333/webhook",
      eventTypes: ["account.created"],
      isActive: false,
      retryCount: 5,
      timeoutSeconds: 60,
    })) as WebhookObject;
    expect(webhook.secret).toBe(secret);
    expect(webhook.is_active).toBe(false);
  });
});

describe("Webhook Delivery", () => {
  let webhookUid: string;
  let accountId: string;
  let webhookServer: { getLastEvent: () => any; close: () => void };

  beforeAll(async () => {
    webhookServer = startWebhookServer(3333);
    const webhook = (await createWebhook({
      isActive: true,
      url: "http://localhost:3333/webhook",
      eventTypes: ["account.created", "event.created"],
    })) as WebhookObject;
    expect(webhook.webhook_uid).toBeDefined();
    webhookUid = webhook.webhook_uid;
  });

  afterAll(async () => {
    webhookServer.close();

    await deleteAccount({ accountId });
    await deleteWebhook({ webhookUid });
  });

  test("Should get webhook deliveries for account created", async () => {
    const account = (await createAccount({
      accountId: crypto.randomUUID(),
      settings: {},
      metadata: {},
    })) as AccountObject;
    expect(account.account_id).toBeDefined();
    expect(account.settings).toEqual({});
    expect(account.metadata).toEqual({});
    accountId = account.account_id;

    await wait(2);

    const deliveries = webhookServer.getLastEvent();
    expect(deliveries).toBeDefined();
    expect(deliveries.event_type).toBe("account.created");
    expect(deliveries.data.account_id).toBe(account.account_id);

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 10,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;
    expect(deliveriesResponse.count).toBe(1);
    expect(deliveriesResponse.data[0].event_type).toBe("account.created");
    expect(deliveriesResponse.data[0].payload.data.account_id).toBe(
      account.account_id
    );
  });

  test("Should get webhook deliveries for event created", async () => {
    const calendar = (await createCalendar({
      accountId,
      settings: {},
      metadata: {},
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();

    const startTs = Math.floor(Date.now() / 1000);
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid: calendar.calendar_uid,
      startTs,
      endTs,
      metadata: {},
    })) as EventObject;
    expect(event.calendar_uid).toBe(calendar.calendar_uid);
    expect(event.event_uid).toBeDefined();
    expect(event.start_ts).toBe(startTs);
    expect(event.end_ts).toBe(endTs);
    expect(event.metadata).toEqual({});

    await wait(2);

    const deliveries = webhookServer.getLastEvent();
    expect(deliveries).toBeDefined();
    expect(deliveries.event_type).toBe("event.created");
    expect(deliveries.data.event_uid).toBe(event.event_uid);

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 10,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;
    expect(deliveriesResponse.count).toBeGreaterThan(0);

    expect(
      deliveriesResponse.data.some((d) => d.event_type === "event.created")
    ).toBe(true);
    expect(
      deliveriesResponse.data.some(
        (d) => d.payload.data.event_uid === event.event_uid
      )
    ).toBe(true);
  });
});

describe("Reminder Webhook Delivery", () => {
  let webhookUid: string;
  let accountId: string;
  let calendarUid: string;
  let webhookServer: { getLastEvent: () => any; close: () => void };

  beforeAll(async () => {
    webhookServer = startWebhookServer(3334);
    const webhook = (await createWebhook({
      isActive: true,
      url: "http://localhost:3334/webhook",
      eventTypes: [
        "reminder.created",
        "reminder.updated",
        "reminder.deleted",
        "reminder.triggered",
      ],
    })) as WebhookObject;
    expect(webhook.webhook_uid).toBeDefined();
    webhookUid = webhook.webhook_uid;

    // Create account and calendar for tests
    const account = (await createAccount({
      accountId: crypto.randomUUID(),
      settings: {},
      metadata: {},
    })) as AccountObject;
    expect(account.account_id).toBeDefined();
    accountId = account.account_id;

    const calendar = (await createCalendar({
      accountId,
      settings: {},
      metadata: {},
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();
    calendarUid = calendar.calendar_uid;
  });

  afterAll(async () => {
    webhookServer.close();
    await deleteAccount({ accountId });
    await deleteWebhook({ webhookUid });
  });

  test("Should get webhook delivery for reminder.created", async () => {
    const startTs = Math.floor(Date.now() / 1000) + 3600;
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid,
      startTs,
      endTs,
      metadata: { title: "Event with Reminder" },
    })) as EventObject;
    expect(event.event_uid).toBeDefined();

    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId,
      offsetSeconds: -900, // 15 minutes before (negative)
      metadata: { type: "notification" },
      scope: "single",
    })) as any;
    expect(response.reminder).toBeDefined();
    const reminder = response.reminder as ReminderObject;
    expect(reminder.reminder_uid).toBeDefined();

    await wait(2);

    const delivery = webhookServer.getLastEvent();
    expect(delivery).toBeDefined();
    expect(delivery.event_type).toBe("reminder.created");
    expect(delivery.data.reminder.reminder_uid).toBe(reminder.reminder_uid);
    expect(delivery.data.event_uid).toBe(event.event_uid);
    expect(delivery.data.reminder.offset_seconds).toBe(-900);

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 10,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;
    expect(deliveriesResponse.count).toBeGreaterThan(0);
    expect(
      deliveriesResponse.data.some((d) => d.event_type === "reminder.created")
    ).toBe(true);
    expect(
      deliveriesResponse.data.some(
        (d) =>
          (d.payload.data as any).reminder?.reminder_uid ===
          reminder.reminder_uid
      )
    ).toBe(true);
  });

  test("Should get webhook delivery for reminder.updated", async () => {
    const startTs = Math.floor(Date.now() / 1000) + 7200;
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid,
      startTs,
      endTs,
      metadata: { title: "Event for Update Test" },
    })) as EventObject;
    expect(event.event_uid).toBeDefined();

    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId,
      offsetSeconds: -1800,
      metadata: { type: "email" },
      scope: "single",
    })) as any;
    expect(response.reminder).toBeDefined();
    const reminder = response.reminder as ReminderObject;
    expect(reminder.reminder_uid).toBeDefined();

    await wait(2);

    const updateResponse = (await updateReminder({
      eventUid: event.event_uid,
      reminderUid: reminder.reminder_uid,
      offsetSeconds: -3600, // Changed to 1 hour before
      metadata: { type: "sms" },
      scope: "single",
    })) as any;
    expect(updateResponse.reminder).toBeDefined();
    const updatedReminder = updateResponse.reminder as ReminderObject;
    expect(updatedReminder.offset_seconds).toBe(-3600);

    await wait(2);

    const delivery = webhookServer.getLastEvent();
    expect(delivery).toBeDefined();
    expect(delivery.event_type).toBe("reminder.updated");
    expect(delivery.data.reminder.reminder_uid).toBe(reminder.reminder_uid);
    expect(delivery.data.reminder.offset_seconds).toBe(-3600);

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 10,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;
    expect(
      deliveriesResponse.data.some((d) => d.event_type === "reminder.updated")
    ).toBe(true);
  });

  test("Should get webhook delivery for reminder.deleted", async () => {
    const startTs = Math.floor(Date.now() / 1000) + 10800;
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid,
      startTs,
      endTs,
      metadata: { title: "Event for Delete Test" },
    })) as EventObject;
    expect(event.event_uid).toBeDefined();

    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId,
      offsetSeconds: -600,
      metadata: { type: "push" },
      scope: "single",
    })) as any;
    expect(response.reminder).toBeDefined();
    const reminder = response.reminder as ReminderObject;
    expect(reminder.reminder_uid).toBeDefined();

    await wait(2);

    const deleteResponse = (await deleteReminder({
      eventUid: event.event_uid,
      reminderUid: reminder.reminder_uid,
      scope: "single",
    })) as any;
    expect(deleteResponse.message).toBeDefined();

    await wait(2);

    const delivery = webhookServer.getLastEvent();
    expect(delivery).toBeDefined();
    expect(delivery.event_type).toBe("reminder.deleted");
    expect(delivery.data.reminder_uid).toBe(reminder.reminder_uid);
    expect(delivery.data.event_uid).toBe(event.event_uid);

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 10,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;
    expect(
      deliveriesResponse.data.some((d) => d.event_type === "reminder.deleted")
    ).toBe(true);
  });

  test("Should get webhook delivery for reminder.created on recurring event", async () => {
    const startTs = Math.floor(Date.now() / 1000) + 14400;
    const endTs = startTs + 3600;
    const recurringEvent = (await createEvent({
      calendarUid,
      startTs,
      endTs,
      metadata: { title: "Recurring Event with Reminder" },
      recurrence: { rule: "FREQ=DAILY;COUNT=3" },
    })) as EventObject;
    expect(recurringEvent.event_uid).toBeDefined();
    expect(recurringEvent.recurrence).toBeDefined();

    // Create reminder for all instances
    const response = (await createReminder({
      eventUid: recurringEvent.event_uid,
      accountId,
      offsetSeconds: -1800,
      metadata: { type: "recurring_notification" },
      scope: "all",
    })) as any;
    expect(response.reminder).toBeDefined();
    expect(response.count).toBeGreaterThan(0);

    await wait(2);

    const delivery = webhookServer.getLastEvent();
    expect(delivery).toBeDefined();
    expect(delivery.event_type).toBe("reminder.created");
    expect(delivery.data).toBeDefined();

    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 20,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    // Should have multiple reminder.created events for each instance
    const reminderCreatedEvents = deliveriesResponse.data.filter(
      (d) => d.event_type === "reminder.created"
    );
    expect(reminderCreatedEvents.length).toBeGreaterThan(0);
  });

  test("Should not get reminder webhook trigger on cancelled events", async () => {
    // Create an event with a reminder that's due very soon
    const startTs = Math.floor(Date.now() / 1000) + 10; // 10 seconds from now
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid,
      startTs,
      endTs,
      metadata: { title: "Event to be Cancelled" },
    })) as EventObject;
    expect(event.event_uid).toBeDefined();

    // Create a reminder that will be due in 5 seconds
    const response = (await createReminder({
      eventUid: event.event_uid,
      accountId,
      offsetSeconds: -5, // 5 seconds before event
      metadata: { type: "test_cancellation" },
      scope: "single",
    })) as any;
    expect(response.reminder).toBeDefined();
    const reminder = response.reminder as ReminderObject;
    expect(reminder.reminder_uid).toBeDefined();

    await wait(2);

    // Cancel the event
    const cancelledEvent = (await toggleCancelledStatusEvent({
      eventUid: event.event_uid,
    })) as EventObject;
    expect(cancelledEvent.is_cancelled).toBe(true);

    // Wait for the reminder to be "due" (but should not trigger)
    await wait(8);

    // Check that no reminder.triggered webhook was sent
    const delivery = webhookServer.getLastEvent();
    expect(delivery).toBeDefined();
    // The last event should NOT be a reminder.triggered for this event
    if (delivery.event_type === "reminder.triggered") {
      expect(delivery.data.event_uid).not.toBe(event.event_uid);
    }

    // Verify in webhook deliveries that no reminder.triggered exists for this event
    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 50,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const triggeredForThisEvent = deliveriesResponse.data.filter(
      (d) =>
        d.event_type === "reminder.triggered" &&
        (d.payload.data as any).event?.event_uid === event.event_uid
    );
    expect(triggeredForThisEvent.length).toBe(0);
  });
});
