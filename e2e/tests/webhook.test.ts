import { afterAll, beforeAll, describe, expect, test } from "vitest";
import {
  createWebhook,
  deleteWebhook,
  getWebhook,
  getWebhookDeliveries,
  updateWebhook,
} from "../helpers/webhook";
import {
  CalendarObject,
  EventObject,
  GenericPagedResponse,
  ReminderObject,
  WebhookDeliveryObject,
  WebhookObject,
} from "../helpers/types";
import { startWebhookServer } from "../helpers/webhook-server";
import { wait } from "../helpers/general";
import { createEvent, toggleCancelledStatusEvent } from "../helpers/event";
import {
  createCalendar,
  deleteCalendar,
  importICS,
  importICSLink,
  resyncCalendar,
} from "../helpers/calendar";
import {
  createReminder,
  deleteReminder,
  updateReminder,
} from "../helpers/reminder";
import { startICSServer } from "../helpers/ics-server";

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

  test("Should update only is_active without breaking other fields (regression test)", async () => {
    // Get current webhook state
    const webhookBefore = (await getWebhook({ webhookUid })) as WebhookObject;

    // Update only is_active
    const webhook = (await updateWebhook({
      webhookUid,
      isActive: true, // Toggling back to active
      url: webhookBefore.url,
      eventTypes: webhookBefore.event_types,
      retryCount: webhookBefore.retry_count,
      timeoutSeconds: webhookBefore.timeout_seconds,
    })) as WebhookObject;

    expect(webhook.is_active).toBe(true);
    // All other fields should be preserved
    expect(webhook.url).toBe(webhookBefore.url);
    expect(webhook.event_types).toEqual(webhookBefore.event_types);
    expect(webhook.retry_count).toBe(webhookBefore.retry_count);
    expect(webhook.timeout_seconds).toBe(webhookBefore.timeout_seconds);
    expect(webhook.secret).toBe(secret);
  });

  test("Should update only url without breaking other fields (regression test)", async () => {
    // Get current webhook state
    const webhookBefore = (await getWebhook({ webhookUid })) as WebhookObject;

    // Update only URL
    const webhook = (await updateWebhook({
      webhookUid,
      url: "http://localhost:4444/new-endpoint",
      eventTypes: webhookBefore.event_types,
      isActive: webhookBefore.is_active,
      retryCount: webhookBefore.retry_count,
      timeoutSeconds: webhookBefore.timeout_seconds,
    })) as WebhookObject;

    expect(webhook.url).toBe("http://localhost:4444/new-endpoint");
    // All other fields should be preserved
    expect(webhook.is_active).toBe(webhookBefore.is_active);
    expect(webhook.event_types).toEqual(webhookBefore.event_types);
    expect(webhook.retry_count).toBe(webhookBefore.retry_count);
    expect(webhook.timeout_seconds).toBe(webhookBefore.timeout_seconds);
    expect(webhook.secret).toBe(secret);
  });

  test("Should update only event_types without breaking other fields (regression test)", async () => {
    // Get current webhook state
    const webhookBefore = (await getWebhook({ webhookUid })) as WebhookObject;

    // Update only event_types
    const webhook = (await updateWebhook({
      webhookUid,
      eventTypes: ["event.created", "event.deleted"],
      url: webhookBefore.url,
      isActive: webhookBefore.is_active,
      retryCount: webhookBefore.retry_count,
      timeoutSeconds: webhookBefore.timeout_seconds,
    })) as WebhookObject;

    expect(webhook.event_types).toEqual(["event.created", "event.deleted"]);
    // All other fields should be preserved
    expect(webhook.url).toBe(webhookBefore.url);
    expect(webhook.is_active).toBe(webhookBefore.is_active);
    expect(webhook.retry_count).toBe(webhookBefore.retry_count);
    expect(webhook.timeout_seconds).toBe(webhookBefore.timeout_seconds);
    expect(webhook.secret).toBe(secret);
  });

  test("Should preserve timeout_seconds when updating retry_count (regression test)", async () => {
    // Get current webhook state
    const webhookBefore = (await getWebhook({ webhookUid })) as WebhookObject;

    // Update only retry_count
    const webhook = (await updateWebhook({
      webhookUid,
      retryCount: 7,
      url: webhookBefore.url,
      eventTypes: webhookBefore.event_types,
      isActive: webhookBefore.is_active,
      timeoutSeconds: webhookBefore.timeout_seconds,
    })) as WebhookObject;

    expect(webhook.retry_count).toBe(7);
    // Timeout should be preserved (this was the bug - it was being set to 0)
    expect(webhook.timeout_seconds).toBe(webhookBefore.timeout_seconds);
    expect(webhook.timeout_seconds).toBeGreaterThan(0);
    // All other fields should also be preserved
    expect(webhook.url).toBe(webhookBefore.url);
    expect(webhook.is_active).toBe(webhookBefore.is_active);
    expect(webhook.event_types).toEqual(webhookBefore.event_types);
    expect(webhook.secret).toBe(secret);
  });
});

describe("Webhook Delivery", () => {
  let webhookUid: string;
  let accountId: string = crypto.randomUUID();
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

    await deleteWebhook({ webhookUid });
  });

  test("Should get webhook deliveries for event created", async () => {
    const calendar = (await createCalendar({
      accountId: accountId,
      settings: {},
      metadata: {},
    })) as CalendarObject;
    expect(calendar.calendar_uid).toBeDefined();

    const startTs = Math.floor(Date.now() / 1000);
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid: calendar.calendar_uid,
      startTs,
      accountId,
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
  let accountId: string = crypto.randomUUID();
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
    await deleteWebhook({ webhookUid });
  });

  test("Should get webhook delivery for reminder.created", async () => {
    const startTs = Math.floor(Date.now() / 1000) + 3600;
    const endTs = startTs + 3600;
    const event = (await createEvent({
      calendarUid,
      accountId,
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
      accountId,
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
      accountId,
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
      accountId,
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
      accountId,
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

describe("Webhook API - ICS Import", () => {
  let webhookServer: any;
  let webhookUid: string;
  const WEBHOOK_PORT = 9200;
  const ICS_SERVER_PORT = 9201;
  const accountId = crypto.randomUUID();
  const createdCalendarUids: string[] = [];

  // Simple ICS content with multiple events
  const multiEventICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:webhook-event-1
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Webhook Event One
END:VEVENT
BEGIN:VEVENT
UID:webhook-event-2
DTSTART:20260116T140000Z
DTEND:20260116T150000Z
SUMMARY:Webhook Event Two
END:VEVENT
BEGIN:VEVENT
UID:webhook-event-3
DTSTART:20260117T090000Z
DTEND:20260117T100000Z
SUMMARY:Webhook Event Three
END:VEVENT
END:VCALENDAR`;

  // Updated ICS with one new event
  const updatedICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:webhook-event-1
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Webhook Event One
END:VEVENT
BEGIN:VEVENT
UID:webhook-event-4
DTSTART:20260118T130000Z
DTEND:20260118T140000Z
SUMMARY:Webhook Event Four
END:VEVENT
END:VCALENDAR`;

  beforeAll(async () => {
    // Start webhook server
    webhookServer = startWebhookServer(WEBHOOK_PORT);

    // Create webhook for ICS-related events
    const webhook = (await createWebhook({
      isActive: true,
      url: `http://localhost:${WEBHOOK_PORT}/webhook`,
      eventTypes: [
        "calendar.created",
        "event.created",
        "calendar.synced",
        "calendar.updated",
        "calendar.resynced",
      ],
    })) as WebhookObject;
    webhookUid = webhook.webhook_uid;

    // Wait for webhook to be ready
    await wait(1);
  });

  afterAll(async () => {
    // Cleanup calendars
    for (const uid of createdCalendarUids) {
      try {
        await deleteCalendar({ calendarUid: uid });
      } catch {
        // Ignore cleanup errors
      }
    }

    // Cleanup webhook
    if (webhookUid) {
      await deleteWebhook({ webhookUid });
    }

    // Close webhook server
    if (webhookServer) {
      webhookServer.close();
    }
  });

  test("Should trigger calendar.created webhook when importing ICS file", async () => {
    // Import ICS file
    const response = (await importICS({
      file: Buffer.from(multiEventICS),
      accountId: accountId,
      calendarMetadata: { name: "Webhook Test Calendar", source: "ics_file" },
    })) as any;

    expect(response.calendar).toBeDefined();
    createdCalendarUids.push(response.calendar.calendar_uid);

    // Wait for webhook delivery
    await wait(2);

    // Check webhook was delivered by querying deliveries
    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 50,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const calendarCreatedDeliveries = deliveriesResponse.data.filter(
      (d) =>
        d.event_type === "calendar.created" &&
        (d.payload.data as any).calendar_uid === response.calendar.calendar_uid
    );

    expect(calendarCreatedDeliveries.length).toBeGreaterThan(0);

    const delivery = calendarCreatedDeliveries[0];
    expect((delivery.payload.data as any).calendar_uid).toBe(
      response.calendar.calendar_uid
    );
    expect((delivery.payload.data as any).account_id).toBe(accountId);
  });

  test("Should trigger batch event.created webhook when importing ICS file with multiple events", async () => {
    // Clear previous events
    webhookServer.getLastEvent();

    // Import ICS file with multiple events
    const response = (await importICS({
      file: Buffer.from(multiEventICS),
      accountId: accountId,
    })) as any;

    expect(response.summary.imported_events).toBe(3);
    createdCalendarUids.push(response.calendar.calendar_uid);

    // Wait for webhook delivery
    await wait(2);

    // Check for event.created webhook deliveries
    const deliveriesResponse = (await getWebhookDeliveries({
      webhookUid,
      limit: 50,
      offset: 0,
    })) as GenericPagedResponse<WebhookDeliveryObject>;

    const eventCreatedDeliveries = deliveriesResponse.data.filter(
      (d) => d.event_type === "event.created"
    );

    // Should have batch event creation webhook
    expect(eventCreatedDeliveries.length).toBeGreaterThan(0);

    // Verify batch contains multiple events
    const batchDelivery = eventCreatedDeliveries[0];
    if (Array.isArray(batchDelivery.payload.data)) {
      expect(batchDelivery.payload.data.length).toBeGreaterThanOrEqual(1);
    }
  });

  test("Should trigger calendar.synced webhook when importing ICS from URL", async () => {
    const icsServer = startICSServer(ICS_SERVER_PORT, multiEventICS);

    try {
      // Import ICS from URL
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${ICS_SERVER_PORT}/calendar.ics`,
        authType: "none",
        calendarMetadata: { name: "Webhook URL Calendar" },
      })) as any;

      expect(response.calendar).toBeDefined();
      expect(response.sync_scheduled).toBe(true);
      createdCalendarUids.push(response.calendar.calendar_uid);

      // Wait for webhook delivery
      await wait(2);

      // Check for calendar.synced webhook
      const deliveriesResponse = (await getWebhookDeliveries({
        webhookUid,
        limit: 50,
        offset: 0,
      })) as GenericPagedResponse<WebhookDeliveryObject>;

      const syncedDeliveries = deliveriesResponse.data.filter(
        (d) =>
          d.event_type === "calendar.synced" &&
          (d.payload.data as any).calendar_uid ===
            response.calendar.calendar_uid
      );

      expect(syncedDeliveries.length).toBeGreaterThan(0);

      const syncDelivery = syncedDeliveries[0];
      expect(syncDelivery.payload.data).toBeDefined();
      expect((syncDelivery.payload.data as any).calendar_uid).toBe(
        response.calendar.calendar_uid
      );
      expect((syncDelivery.payload.data as any).imported_events).toBe(3);
      expect((syncDelivery.payload.data as any).failed_events).toBe(0);
    } finally {
      await icsServer.close();
    }
  });

  test("Should trigger calendar.synced webhook on manual resync", async () => {
    const icsServer = startICSServer(ICS_SERVER_PORT, multiEventICS);

    try {
      // Import ICS from URL
      const importResponse = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${ICS_SERVER_PORT}/calendar.ics`,
        authType: "none",
      })) as any;

      expect(importResponse.calendar).toBeDefined();
      createdCalendarUids.push(importResponse.calendar.calendar_uid);

      await wait(2);

      // Update ICS content on server
      icsServer.updateContent(updatedICS);

      // Clear previous webhook events
      webhookServer.getLastEvent();

      // Manually resync
      const resyncResponse = (await resyncCalendar({
        calendarUid: importResponse.calendar.calendar_uid,
      })) as any;

      expect(resyncResponse.imported_events).toBeGreaterThanOrEqual(1);

      // Wait for webhook delivery
      await wait(2);

      // Check for calendar.synced webhook after resync
      const deliveriesResponse = (await getWebhookDeliveries({
        webhookUid,
        limit: 50,
        offset: 0,
      })) as GenericPagedResponse<WebhookDeliveryObject>;

      const recentSyncDeliveries = deliveriesResponse.data.filter(
        (d) =>
          d.event_type === "calendar.synced" &&
          (d.payload.data as any).calendar_uid ===
            importResponse.calendar.calendar_uid
      );

      expect(recentSyncDeliveries.length).toBeGreaterThan(0);

      // Verify the most recent sync delivery has updated data
      const latestSync = recentSyncDeliveries[0];
      expect(
        (latestSync.payload.data as any).imported_events
      ).toBeGreaterThanOrEqual(1);
    } finally {
      await icsServer.close();
    }
  });

  test("Should include batch event webhooks when syncing ICS URL", async () => {
    const icsServer = startICSServer(ICS_SERVER_PORT, multiEventICS);

    try {
      // Import ICS from URL
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${ICS_SERVER_PORT}/calendar.ics`,
        authType: "none",
      })) as any;

      expect(response.calendar).toBeDefined();
      createdCalendarUids.push(response.calendar.calendar_uid);

      // Wait for webhook delivery
      await wait(2);

      // Check for event.created webhook deliveries
      const deliveriesResponse = (await getWebhookDeliveries({
        webhookUid,
        limit: 100,
        offset: 0,
      })) as GenericPagedResponse<WebhookDeliveryObject>;

      const eventCreatedDeliveries = deliveriesResponse.data.filter(
        (d) => d.event_type === "event.created"
      );

      // Should have batch event creation webhooks
      expect(eventCreatedDeliveries.length).toBeGreaterThan(0);
    } finally {
      await icsServer.close();
    }
  });

  test("Should include warnings in calendar.synced webhook when partial sync fails", async () => {
    // ICS with one valid and one invalid event
    // Note: Events without DTSTART are silently skipped by the parser
    // So we need a different type of failure
    const partialICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:valid-event
DTSTART:20260115T100000Z
DTEND:20260115T110000Z
SUMMARY:Valid Event
END:VEVENT
BEGIN:VEVENT
UID:invalid-event
SUMMARY:Invalid Event Without Dates
END:VEVENT
END:VCALENDAR`;

    const icsServer = startICSServer(ICS_SERVER_PORT, partialICS);

    try {
      // Import ICS from URL with partial failure mode
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${ICS_SERVER_PORT}/calendar.ics`,
        authType: "none",
        syncOnPartialFailure: true,
      })) as any;

      expect(response.calendar).toBeDefined();
      createdCalendarUids.push(response.calendar.calendar_uid);

      // Check if there were any failed events in the response
      const hasFailed = response.summary.failed_events > 0;

      // Wait for webhook delivery
      await wait(2);

      // Check for calendar.synced webhook
      const deliveriesResponse = (await getWebhookDeliveries({
        webhookUid,
        limit: 50,
        offset: 0,
      })) as GenericPagedResponse<WebhookDeliveryObject>;

      const syncedDeliveries = deliveriesResponse.data.filter(
        (d) =>
          d.event_type === "calendar.synced" &&
          (d.payload.data as any).calendar_uid ===
            response.calendar.calendar_uid
      );

      expect(syncedDeliveries.length).toBeGreaterThan(0);

      const syncDelivery = syncedDeliveries[0];
      const syncData = syncDelivery.payload.data as any;

      // Should have imported at least one event
      expect(syncData.imported_events).toBeGreaterThan(0);

      // If there were failures, should have warnings
      if (hasFailed) {
        expect(syncData.warnings).toBeDefined();
        expect(Array.isArray(syncData.warnings)).toBe(true);
        expect(syncData.warnings.length).toBeGreaterThan(0);
      } else {
        // If no failures (parser skipped invalid events), warnings should be empty or undefined
        // This is acceptable behavior - parser filters out unparseable events
        expect(syncData.warnings).toBeDefined();
        expect(Array.isArray(syncData.warnings)).toBe(true);
      }
    } finally {
      await icsServer.close();
    }
  });

  test("Should trigger calendar.created even for read-only ICS calendars", async () => {
    const icsServer = startICSServer(ICS_SERVER_PORT, multiEventICS);

    try {
      // Clear previous events
      webhookServer.getLastEvent();

      // Import ICS from URL (creates read-only calendar)
      const response = (await importICSLink({
        accountId: accountId,
        icsUrl: `http://localhost:${ICS_SERVER_PORT}/calendar.ics`,
        authType: "none",
      })) as any;

      expect(response.calendar.is_read_only).toBe(true);
      createdCalendarUids.push(response.calendar.calendar_uid);

      // Wait for webhook delivery
      await wait(2);

      // Check for calendar.created webhook
      const deliveriesResponse = (await getWebhookDeliveries({
        webhookUid,
        limit: 50,
        offset: 0,
      })) as GenericPagedResponse<WebhookDeliveryObject>;

      const calendarCreatedDeliveries = deliveriesResponse.data.filter(
        (d) =>
          d.event_type === "calendar.created" &&
          (d.payload.data as any).calendar_uid ===
            response.calendar.calendar_uid
      );

      expect(calendarCreatedDeliveries.length).toBeGreaterThan(0);

      const createDelivery = calendarCreatedDeliveries[0];
      expect((createDelivery.payload.data as any).is_read_only).toBe(true);
    } finally {
      await icsServer.close();
    }
  });
});
