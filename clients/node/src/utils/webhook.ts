import { createHmac, timingSafeEqual } from "crypto";
import { WebhookPayload, WebhookVerificationResult } from "../types";

/**
 * Webhook utility class for verifying and handling webhook payloads
 */
export class WebhookUtils {
  /**
   * Verify a webhook signature using HMAC-SHA256
   * @param payload - The raw webhook payload (as string or Buffer)
   * @param signature - The signature from the X-Webhook-Signature header
   * @param secret - The webhook secret
   * @returns Verification result with validity and optional error message
   */
  static verifySignature(
    payload: string | Buffer,
    signature: string,
    secret: string
  ): WebhookVerificationResult {
    try {
      // Normalize payload to string
      const payloadString =
        typeof payload === "string" ? payload : payload.toString("utf8");

      // Extract the signature value (format: "sha256=<hex>")
      if (!signature || !signature.startsWith("sha256=")) {
        return {
          valid: false,
          error: "Invalid signature format. Expected 'sha256=<hex>'",
        };
      }

      const providedSignature = signature.substring(7); // Remove "sha256=" prefix

      // Compute expected signature
      const expectedSignature = createHmac("sha256", secret)
        .update(payloadString)
        .digest("hex");

      // Use timing-safe comparison to prevent timing attacks
      const providedBuffer = Buffer.from(providedSignature, "hex");
      const expectedBuffer = Buffer.from(expectedSignature, "hex");

      if (providedBuffer.length !== expectedBuffer.length) {
        return {
          valid: false,
          error: "Signature length mismatch",
        };
      }

      const isValid = timingSafeEqual(providedBuffer, expectedBuffer);

      return {
        valid: isValid,
        error: isValid ? undefined : "Signature verification failed",
      };
    } catch (error) {
      return {
        valid: false,
        error: `Verification error: ${
          error instanceof Error ? error.message : String(error)
        }`,
      };
    }
  }

  /**
   * Compute the signature for a webhook payload
   * Useful for testing webhook endpoints
   * @param payload - The webhook payload object or string
   * @param secret - The webhook secret
   * @returns Signature in the format "sha256=<hex>"
   */
  static computeSignature(
    payload: WebhookPayload | string | Buffer,
    secret: string
  ): string {
    const payloadString =
      typeof payload === "string"
        ? payload
        : Buffer.isBuffer(payload)
        ? payload.toString("utf8")
        : JSON.stringify(payload);

    const signature = createHmac("sha256", secret)
      .update(payloadString)
      .digest("hex");

    return `sha256=${signature}`;
  }

  /**
   * Parse and verify a webhook payload
   * @param rawPayload - The raw webhook payload (as received from request)
   * @param signature - The signature from the X-Webhook-Signature header
   * @param secret - The webhook secret
   * @returns Parsed payload if valid, null otherwise
   */
  static parseAndVerify<T = any>(
    rawPayload: string | Buffer,
    signature: string,
    secret: string
  ): { payload: WebhookPayload<T>; error?: string } | null {
    // Verify signature first
    const verification = this.verifySignature(rawPayload, signature, secret);

    if (!verification.valid) {
      return null;
    }

    // Parse payload
    try {
      const payloadString =
        typeof rawPayload === "string"
          ? rawPayload
          : rawPayload.toString("utf8");
      const payload = JSON.parse(payloadString) as WebhookPayload<T>;

      return { payload };
    } catch (error) {
      return null;
    }
  }

  /**
   * Validate webhook payload structure
   * @param payload - The parsed webhook payload
   * @returns True if payload has required fields
   */
  static validatePayloadStructure(payload: any): payload is WebhookPayload {
    return (
      typeof payload === "object" &&
      payload !== null &&
      typeof payload.webhook_uid === "string" &&
      typeof payload.event_type === "string" &&
      typeof payload.delivery_id === "string" &&
      typeof payload.timestamp === "number" &&
      "data" in payload
    );
  }

  /**
   * Check if a webhook timestamp is fresh (within acceptable time window)
   * Helps prevent replay attacks
   * @param timestamp - The webhook timestamp (Unix timestamp in seconds)
   * @param toleranceSeconds - Maximum age of webhook in seconds (default: 300 = 5 minutes)
   * @returns True if timestamp is within tolerance
   */
  static isTimestampFresh(
    timestamp: number,
    toleranceSeconds: number = 300
  ): boolean {
    const now = Math.floor(Date.now() / 1000);
    const age = Math.abs(now - timestamp);
    return age <= toleranceSeconds;
  }

  /**
   * Verify both signature and timestamp freshness
   * @param rawPayload - The raw webhook payload
   * @param signature - The signature from the X-Webhook-Signature header
   * @param secret - The webhook secret
   * @param toleranceSeconds - Maximum age of webhook in seconds (default: 300)
   * @returns Verification result
   */
  static verifyWithTimestamp(
    rawPayload: string | Buffer,
    signature: string,
    secret: string,
    toleranceSeconds: number = 300
  ): WebhookVerificationResult {
    // Verify signature
    const signatureResult = this.verifySignature(rawPayload, signature, secret);

    if (!signatureResult.valid) {
      return signatureResult;
    }

    // Parse and check timestamp
    try {
      const payloadString =
        typeof rawPayload === "string"
          ? rawPayload
          : rawPayload.toString("utf8");
      const payload = JSON.parse(payloadString) as WebhookPayload;

      if (!this.isTimestampFresh(payload.timestamp, toleranceSeconds)) {
        return {
          valid: false,
          error: `Webhook timestamp is too old. Age exceeds ${toleranceSeconds} seconds.`,
        };
      }

      return { valid: true };
    } catch (error) {
      return {
        valid: false,
        error: "Failed to parse payload for timestamp verification",
      };
    }
  }

  /**
   * Extract webhook signature from request headers
   * Supports multiple header formats
   * @param headers - Request headers object
   * @returns Signature string or null if not found
   */
  static extractSignature(
    headers: Record<string, string | string[] | undefined>
  ): string | null {
    // Try different header formats
    const signature =
      headers["x-webhook-signature"] ||
      headers["X-Webhook-Signature"] ||
      headers["x-signature"] ||
      headers["X-Signature"];

    if (Array.isArray(signature)) {
      return signature[0] || null;
    }

    return signature || null;
  }

  /**
   * Create a webhook response helper for Express/HTTP frameworks
   * @param statusCode - HTTP status code to return
   * @param message - Optional message
   * @returns Response object
   */
  static createResponse(statusCode: number, message?: string) {
    return {
      statusCode,
      body: message ? JSON.stringify({ message }) : undefined,
    };
  }

  /**
   * Success response for webhook acknowledgment
   */
  static success() {
    return this.createResponse(200, "Webhook received");
  }

  /**
   * Error response for invalid signature
   */
  static invalidSignature() {
    return this.createResponse(401, "Invalid webhook signature");
  }

  /**
   * Error response for stale timestamp
   */
  static staleTimestamp() {
    return this.createResponse(400, "Webhook timestamp is too old");
  }

  /**
   * Error response for invalid payload
   */
  static invalidPayload() {
    return this.createResponse(400, "Invalid webhook payload");
  }
}

/**
 * Express middleware factory for webhook verification
 * @param secret - The webhook secret
 * @param options - Optional configuration
 * @returns Express middleware function
 */
export function createWebhookVerificationMiddleware(
  secret: string,
  options: {
    toleranceSeconds?: number;
    rawBodyKey?: string;
  } = {}
) {
  const { toleranceSeconds = 300, rawBodyKey = "rawBody" } = options;

  return (req: any, res: any, next: any) => {
    try {
      // Extract signature from headers
      const signature = WebhookUtils.extractSignature(req.headers);

      if (!signature) {
        res.status(401).json({ error: "Missing webhook signature" });
        return;
      }

      // Get raw body (must be preserved by body parser)
      const rawBody = req[rawBodyKey] || req.body;

      if (!rawBody) {
        res.status(400).json({ error: "Missing request body" });
        return;
      }

      // Verify signature and timestamp
      const verification = WebhookUtils.verifyWithTimestamp(
        rawBody,
        signature,
        secret,
        toleranceSeconds
      );

      if (!verification.valid) {
        res.status(401).json({ error: verification.error });
        return;
      }

      // Attach parsed payload to request
      try {
        const payloadString =
          typeof rawBody === "string" ? rawBody : rawBody.toString("utf8");
        req.webhookPayload = JSON.parse(payloadString);
      } catch (error) {
        res.status(400).json({ error: "Invalid JSON payload" });
        return;
      }

      next();
    } catch (error) {
      res.status(500).json({
        error: "Webhook verification failed",
        details: error instanceof Error ? error.message : String(error),
      });
    }
  };
}

// Export both the class and individual functions for convenience
export const {
  verifySignature,
  computeSignature,
  parseAndVerify,
  validatePayloadStructure,
  isTimestampFresh,
  verifyWithTimestamp,
  extractSignature,
} = WebhookUtils;
