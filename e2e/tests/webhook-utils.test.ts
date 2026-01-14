import { describe, expect, test, beforeAll, afterAll } from "vitest";
import express from "express";
import {
  WebhookUtils,
  createWebhookVerificationMiddleware,
  WebhookPayload,
  isCalendarWebhook,
  isEventWebhook,
  isReminderDueWebhook,
  isAttendeeWebhook,
  isMemberWebhook,
  isCalendarSyncedWebhook,
} from "go-scheduler-node-sdk";

describe("Webhook Utilities", () => {
  const testSecret = "test-webhook-secret-12345";

  const samplePayload: WebhookPayload = {
    webhook_uid: "webhook-123",
    event_type: "event.created",
    delivery_id: "delivery-456",
    timestamp: Math.floor(Date.now() / 1000),
    data: {
      event_uid: "event-789",
      calendar_uid: "cal-123",
      account_id: "account-456",
      start_ts: 1705970000,
    },
  };

  describe("computeSignature", () => {
    test("Should compute valid signature for string payload", () => {
      const payloadString = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      expect(signature).toBeDefined();
      expect(signature).toMatch(/^sha256=[a-f0-9]{64}$/);
    });

    test("Should compute valid signature for object payload", () => {
      const signature = WebhookUtils.computeSignature(
        samplePayload,
        testSecret,
      );

      expect(signature).toBeDefined();
      expect(signature).toMatch(/^sha256=[a-f0-9]{64}$/);
    });

    test("Should compute valid signature for Buffer payload", () => {
      const payloadBuffer = Buffer.from(JSON.stringify(samplePayload));
      const signature = WebhookUtils.computeSignature(
        payloadBuffer,
        testSecret,
      );

      expect(signature).toBeDefined();
      expect(signature).toMatch(/^sha256=[a-f0-9]{64}$/);
    });

    test("Should compute same signature for same payload", () => {
      const signature1 = WebhookUtils.computeSignature(
        samplePayload,
        testSecret,
      );
      const signature2 = WebhookUtils.computeSignature(
        samplePayload,
        testSecret,
      );

      expect(signature1).toBe(signature2);
    });

    test("Should compute different signatures for different payloads", () => {
      const payload1 = { ...samplePayload, timestamp: 1000 };
      const payload2 = { ...samplePayload, timestamp: 2000 };

      const signature1 = WebhookUtils.computeSignature(payload1, testSecret);
      const signature2 = WebhookUtils.computeSignature(payload2, testSecret);

      expect(signature1).not.toBe(signature2);
    });

    test("Should compute different signatures for different secrets", () => {
      const signature1 = WebhookUtils.computeSignature(
        samplePayload,
        "secret1",
      );
      const signature2 = WebhookUtils.computeSignature(
        samplePayload,
        "secret2",
      );

      expect(signature1).not.toBe(signature2);
    });
  });

  describe("verifySignature", () => {
    test("Should verify valid signature", () => {
      const payloadString = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const result = WebhookUtils.verifySignature(
        payloadString,
        signature,
        testSecret,
      );

      expect(result.valid).toBe(true);
      expect(result.error).toBeUndefined();
    });

    test("Should reject invalid signature", () => {
      const payloadString = JSON.stringify(samplePayload);
      const invalidSignature =
        "sha256=0000000000000000000000000000000000000000000000000000000000000000";

      const result = WebhookUtils.verifySignature(
        payloadString,
        invalidSignature,
        testSecret,
      );

      expect(result.valid).toBe(false);
      expect(result.error).toBeDefined();
    });

    test("Should reject signature with wrong secret", () => {
      const payloadString = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        "wrong-secret",
      );

      const result = WebhookUtils.verifySignature(
        payloadString,
        signature,
        testSecret,
      );

      expect(result.valid).toBe(false);
      expect(result.error).toBe("Signature verification failed");
    });

    test("Should reject signature without sha256 prefix", () => {
      const payloadString = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );
      const signatureWithoutPrefix = signature.substring(7); // Remove "sha256="

      const result = WebhookUtils.verifySignature(
        payloadString,
        signatureWithoutPrefix,
        testSecret,
      );

      expect(result.valid).toBe(false);
      expect(result.error).toContain("Invalid signature format");
    });

    test("Should verify signature for Buffer payload", () => {
      const payloadBuffer = Buffer.from(JSON.stringify(samplePayload));
      const signature = WebhookUtils.computeSignature(
        payloadBuffer,
        testSecret,
      );

      const result = WebhookUtils.verifySignature(
        payloadBuffer,
        signature,
        testSecret,
      );

      expect(result.valid).toBe(true);
    });

    test("Should reject tampered payload", () => {
      const originalPayload = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        originalPayload,
        testSecret,
      );

      const tamperedPayload = JSON.stringify({
        ...samplePayload,
        data: { ...samplePayload.data, account_id: "attacker-account" },
      });

      const result = WebhookUtils.verifySignature(
        tamperedPayload,
        signature,
        testSecret,
      );

      expect(result.valid).toBe(false);
    });
  });

  describe("parseAndVerify", () => {
    test("Should parse and verify valid webhook", () => {
      const payloadString = JSON.stringify(samplePayload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const result = WebhookUtils.parseAndVerify(
        payloadString,
        signature,
        testSecret,
      );

      expect(result).not.toBeNull();
      expect(result?.payload).toEqual(samplePayload);
      expect(result?.error).toBeUndefined();
    });

    test("Should return null for invalid signature", () => {
      const payloadString = JSON.stringify(samplePayload);
      const invalidSignature =
        "sha256=0000000000000000000000000000000000000000000000000000000000000000";

      const result = WebhookUtils.parseAndVerify(
        payloadString,
        invalidSignature,
        testSecret,
      );

      expect(result).toBeNull();
    });

    test("Should return null for malformed JSON", () => {
      const invalidJson = "{ invalid json }";
      const signature = WebhookUtils.computeSignature(invalidJson, testSecret);

      const result = WebhookUtils.parseAndVerify(
        invalidJson,
        signature,
        testSecret,
      );

      expect(result).toBeNull();
    });
  });

  describe("validatePayloadStructure", () => {
    test("Should validate correct webhook payload", () => {
      expect(WebhookUtils.validatePayloadStructure(samplePayload)).toBe(true);
    });

    test("Should reject payload without webhook_uid", () => {
      const invalid = { ...samplePayload };
      delete (invalid as any).webhook_uid;

      expect(WebhookUtils.validatePayloadStructure(invalid)).toBe(false);
    });

    test("Should reject payload without event_type", () => {
      const invalid = { ...samplePayload };
      delete (invalid as any).event_type;

      expect(WebhookUtils.validatePayloadStructure(invalid)).toBe(false);
    });

    test("Should reject payload without delivery_id", () => {
      const invalid = { ...samplePayload };
      delete (invalid as any).delivery_id;

      expect(WebhookUtils.validatePayloadStructure(invalid)).toBe(false);
    });

    test("Should reject payload without timestamp", () => {
      const invalid = { ...samplePayload };
      delete (invalid as any).timestamp;

      expect(WebhookUtils.validatePayloadStructure(invalid)).toBe(false);
    });

    test("Should reject payload without data", () => {
      const invalid = { ...samplePayload };
      delete (invalid as any).data;

      expect(WebhookUtils.validatePayloadStructure(invalid)).toBe(false);
    });

    test("Should reject null payload", () => {
      expect(WebhookUtils.validatePayloadStructure(null)).toBe(false);
    });

    test("Should reject non-object payload", () => {
      expect(WebhookUtils.validatePayloadStructure("not an object")).toBe(
        false,
      );
      expect(WebhookUtils.validatePayloadStructure(123)).toBe(false);
    });
  });

  describe("isTimestampFresh", () => {
    test("Should accept fresh timestamp", () => {
      const now = Math.floor(Date.now() / 1000);
      expect(WebhookUtils.isTimestampFresh(now)).toBe(true);
    });

    test("Should accept timestamp within tolerance", () => {
      const now = Math.floor(Date.now() / 1000);
      const recentTimestamp = now - 200; // 200 seconds ago

      expect(WebhookUtils.isTimestampFresh(recentTimestamp, 300)).toBe(true);
    });

    test("Should reject old timestamp", () => {
      const now = Math.floor(Date.now() / 1000);
      const oldTimestamp = now - 600; // 10 minutes ago

      expect(WebhookUtils.isTimestampFresh(oldTimestamp, 300)).toBe(false);
    });

    test("Should accept custom tolerance", () => {
      const now = Math.floor(Date.now() / 1000);
      const timestamp = now - 500; // 500 seconds ago

      expect(WebhookUtils.isTimestampFresh(timestamp, 600)).toBe(true);
      expect(WebhookUtils.isTimestampFresh(timestamp, 400)).toBe(false);
    });

    test("Should handle future timestamps within tolerance", () => {
      const now = Math.floor(Date.now() / 1000);
      const futureTimestamp = now + 100; // 100 seconds in future

      expect(WebhookUtils.isTimestampFresh(futureTimestamp, 300)).toBe(true);
    });
  });

  describe("verifyWithTimestamp", () => {
    test("Should verify valid signature with fresh timestamp", () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000),
      };
      const payloadString = JSON.stringify(payload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const result = WebhookUtils.verifyWithTimestamp(
        payloadString,
        signature,
        testSecret,
        300,
      );

      expect(result.valid).toBe(true);
      expect(result.error).toBeUndefined();
    });

    test("Should reject valid signature with stale timestamp", () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000) - 600, // 10 minutes ago
      };
      const payloadString = JSON.stringify(payload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const result = WebhookUtils.verifyWithTimestamp(
        payloadString,
        signature,
        testSecret,
        300,
      );

      expect(result.valid).toBe(false);
      expect(result.error).toContain("too old");
    });

    test("Should reject invalid signature even with fresh timestamp", () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000),
      };
      const payloadString = JSON.stringify(payload);
      const invalidSignature =
        "sha256=0000000000000000000000000000000000000000000000000000000000000000";

      const result = WebhookUtils.verifyWithTimestamp(
        payloadString,
        invalidSignature,
        testSecret,
        300,
      );

      expect(result.valid).toBe(false);
    });
  });

  describe("extractSignature", () => {
    test("Should extract signature from x-webhook-signature header", () => {
      const headers = {
        "x-webhook-signature": "sha256=abc123",
      };

      const signature = WebhookUtils.extractSignature(headers);
      expect(signature).toBe("sha256=abc123");
    });

    test("Should extract signature from X-Webhook-Signature header", () => {
      const headers = {
        "X-Webhook-Signature": "sha256=abc123",
      };

      const signature = WebhookUtils.extractSignature(headers);
      expect(signature).toBe("sha256=abc123");
    });

    test("Should extract signature from x-signature header", () => {
      const headers = {
        "x-signature": "sha256=abc123",
      };

      const signature = WebhookUtils.extractSignature(headers);
      expect(signature).toBe("sha256=abc123");
    });

    test("Should return null if no signature header", () => {
      const headers = {
        "content-type": "application/json",
      };

      const signature = WebhookUtils.extractSignature(headers);
      expect(signature).toBeNull();
    });

    test("Should handle array header values", () => {
      const headers = {
        "x-webhook-signature": ["sha256=abc123", "sha256=def456"],
      };

      const signature = WebhookUtils.extractSignature(headers);
      expect(signature).toBe("sha256=abc123");
    });
  });

  describe("Type guards", () => {
    test("isCalendarWebhook should identify calendar webhooks", () => {
      const calendarPayload: WebhookPayload = {
        ...samplePayload,
        event_type: "calendar.created",
      };

      expect(isCalendarWebhook(calendarPayload)).toBe(true);
      expect(isEventWebhook(calendarPayload)).toBe(false);
    });

    test("isCalendarSyncedWebhook should identify synced webhooks", () => {
      const syncedPayload: WebhookPayload = {
        ...samplePayload,
        event_type: "calendar.synced",
      };

      expect(isCalendarSyncedWebhook(syncedPayload)).toBe(true);
      expect(isCalendarWebhook(syncedPayload)).toBe(false);
    });

    test("isEventWebhook should identify event webhooks", () => {
      const eventPayload: WebhookPayload = {
        ...samplePayload,
        event_type: "event.updated",
      };

      expect(isEventWebhook(eventPayload)).toBe(true);
      expect(isCalendarWebhook(eventPayload)).toBe(false);
    });

    test("isReminderDueWebhook should identify reminder webhooks", () => {
      const reminderPayload: WebhookPayload = {
        ...samplePayload,
        event_type: "reminder.due",
      };

      expect(isReminderDueWebhook(reminderPayload)).toBe(true);
      expect(isEventWebhook(reminderPayload)).toBe(false);
    });

    test("isAttendeeWebhook should identify attendee webhooks", () => {
      const attendeePayload: WebhookPayload = {
        ...samplePayload,
        event_type: "attendee.created",
      };

      expect(isAttendeeWebhook(attendeePayload)).toBe(true);
      expect(isEventWebhook(attendeePayload)).toBe(false);
    });

    test("isMemberWebhook should identify member webhooks", () => {
      const memberPayload: WebhookPayload = {
        ...samplePayload,
        event_type: "member.invited",
      };

      expect(isMemberWebhook(memberPayload)).toBe(true);
      expect(isEventWebhook(memberPayload)).toBe(false);
    });
  });

  describe("Response helpers", () => {
    test("success() should return 200 status", () => {
      const response = WebhookUtils.success();
      expect(response.statusCode).toBe(200);
      expect(response.body).toContain("received");
    });

    test("invalidSignature() should return 401 status", () => {
      const response = WebhookUtils.invalidSignature();
      expect(response.statusCode).toBe(401);
      expect(response.body).toContain("Invalid");
    });

    test("staleTimestamp() should return 400 status", () => {
      const response = WebhookUtils.staleTimestamp();
      expect(response.statusCode).toBe(400);
      expect(response.body).toContain("too old");
    });

    test("invalidPayload() should return 400 status", () => {
      const response = WebhookUtils.invalidPayload();
      expect(response.statusCode).toBe(400);
      expect(response.body).toContain("Invalid");
    });
  });

  describe("Express Middleware Integration", () => {
    let app: express.Application;
    let server: any;
    const port = 9200;

    beforeAll(() => {
      app = express();

      // Middleware to preserve raw body
      app.use(
        express.json({
          verify: (req: any, res, buf) => {
            req.rawBody = buf.toString("utf8");
          },
        }),
      );

      // Apply webhook verification middleware
      app.post(
        "/webhook",
        createWebhookVerificationMiddleware(testSecret, {
          rawBodyKey: "rawBody",
        }),
        (req: any, res) => {
          res.status(200).json({
            received: true,
            payload: req.webhookPayload,
          });
        },
      );

      server = app.listen(port);
    });

    afterAll(() => {
      server?.close();
    });

    test("Should accept valid webhook request", async () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000),
      };
      const payloadString = JSON.stringify(payload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const response = await fetch(`http://localhost:${port}/webhook`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Webhook-Signature": signature,
        },
        body: payloadString,
      });

      expect(response.status).toBe(200);
      const data = await response.json();
      expect(data.received).toBe(true);
      expect(data.payload).toEqual(payload);
    });

    test("Should reject request with invalid signature", async () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000),
      };
      const payloadString = JSON.stringify(payload);
      const invalidSignature =
        "sha256=0000000000000000000000000000000000000000000000000000000000000000";

      const response = await fetch(`http://localhost:${port}/webhook`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Webhook-Signature": invalidSignature,
        },
        body: payloadString,
      });

      expect(response.status).toBe(401);
      const data = await response.json();
      expect(data.error).toBeDefined();
    });

    test("Should reject request without signature header", async () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000),
      };
      const payloadString = JSON.stringify(payload);

      const response = await fetch(`http://localhost:${port}/webhook`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: payloadString,
      });

      expect(response.status).toBe(401);
      const data = await response.json();
      expect(data.error).toContain("Missing");
    });

    test("Should reject request with stale timestamp", async () => {
      const payload: WebhookPayload = {
        ...samplePayload,
        timestamp: Math.floor(Date.now() / 1000) - 600, // 10 minutes ago
      };
      const payloadString = JSON.stringify(payload);
      const signature = WebhookUtils.computeSignature(
        payloadString,
        testSecret,
      );

      const response = await fetch(`http://localhost:${port}/webhook`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Webhook-Signature": signature,
        },
        body: payloadString,
      });

      expect(response.status).toBe(401);
      const data = await response.json();
      expect(data.error).toContain("too old");
    });
  });
});
