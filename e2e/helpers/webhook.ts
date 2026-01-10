import {
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
  WebhookDeliveryObject,
  WebhookObject,
} from "./types";

export const createWebhook = async ({
  isActive,
  url,
  eventTypes,
}: {
  isActive: boolean;
  url: string;
  eventTypes: string[];
}) => {
  const body = {
    url,
    event_types: eventTypes,
    is_active: isActive,
  };

  const response = await fetch(`${process.env.SCHEDULER_URL}/api/v1/webhooks`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
    body: JSON.stringify(body),
  });

  return response.json() as Promise<WebhookObject | ErrorObject>;
};

export const getWebhook = async ({ webhookUid }: { webhookUid: string }) => {
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/webhooks/${webhookUid}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
    }
  );

  return response.json() as Promise<WebhookObject | ErrorObject>;
};

export const getWebhookEndpoints = async ({
  limit,
  offset,
}: {
  limit: number;
  offset: number;
}) => {
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/webhooks?limit=${limit}&offset=${offset}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
    }
  );

  return response.json() as Promise<
    GenericPagedResponse<WebhookObject> | ErrorObject
  >;
};

export const updateWebhook = async ({
  webhookUid,
  url,
  eventTypes,
  isActive,
  retryCount,
  timeoutSeconds,
}: {
  webhookUid: string;
  url: string;
  eventTypes: string[];
  isActive: boolean;
  retryCount: number;
  timeoutSeconds: number;
}) => {
  const body = {
    url,
    event_types: eventTypes,
    is_active: isActive,
    retry_count: retryCount,
    timeout_seconds: timeoutSeconds,
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/webhooks/${webhookUid}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<WebhookObject | ErrorObject>;
};

export const getWebhookDeliveries = async ({
  webhookUid,
  limit,
  offset,
}: {
  webhookUid: string;
  limit: number;
  offset: number;
}) => {
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/webhooks/deliveries/${webhookUid}?limit=${limit}&offset=${offset}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
    }
  );

  return response.json() as Promise<
    GenericPagedResponse<WebhookDeliveryObject> | ErrorObject
  >;
};

export const deleteWebhook = async ({ webhookUid }: { webhookUid: string }) => {
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/webhooks/${webhookUid}`,
    {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
    }
  );

  return response.json() as Promise<SuccessObject | ErrorObject>;
};
