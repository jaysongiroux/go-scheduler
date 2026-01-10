import { ErrorObject, ReminderObject, SuccessObject } from "./types";

export const deleteReminder = async ({
  reminderUid,
  eventUid,
  scope,
}: {
  reminderUid: string;
  eventUid: string;
  scope?: "single" | "all";
}) => {
  let url = `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/reminders/${reminderUid}`;
  if (scope) {
    url += `?scope=${scope}`;
  }
  const response = await fetch(url, {
    method: "DELETE",
    headers: {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
  });

  return response.json() as Promise<SuccessObject>;
};

export const createReminder = async ({
  eventUid,
  offsetSeconds,
  accountId,
  metadata,
  scope,
}: {
  eventUid: string;
  accountId: string;
  offsetSeconds: number;
  metadata?: Record<string, unknown>;
  scope?: "single" | "all";
}) => {
  const body = {
    event_uid: eventUid,
    account_id: accountId,
    offset_seconds: offsetSeconds,
    ...(metadata && { metadata }),
    ...(scope && { scope }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/reminders`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<
    ReminderObject | ReminderObject[] | ErrorObject
  >;
};

export const updateReminder = async ({
  reminderUid,
  offsetSeconds,
  metadata,
  eventUid,
  scope,
}: {
  reminderUid: string;
  offsetSeconds: number;
  metadata?: Record<string, unknown>;
  eventUid: string;
  scope?: "single" | "all";
}) => {
  const body = {
    offset_seconds: offsetSeconds,
    ...(metadata && { metadata }),
    ...(scope && { scope }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/reminders/${reminderUid}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<ReminderObject | ErrorObject>;
};

export const getEventReminders = async ({
  eventUid,
  accountId,
}: {
  eventUid: string;
  accountId?: string;
}): Promise<ReminderObject[] | ErrorObject> => {
  let url = `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/reminders`;

  if (accountId) {
    url += `?account_id=${accountId}`;
  }

  const response = await fetch(url.toString(), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
  });

  return await response.json();
};
