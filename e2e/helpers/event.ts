import {
  ErrorObject,
  EventObject,
  GenericPagedResponse,
  SuccessObject,
} from "./types";

export const createEvent = async ({
  calendarUid,
  accountId,
  startTs,
  endTs,
  metadata,
  recurrence,
  timezone,
  localStart,
}: {
  calendarUid: string;
  accountId: string;
  startTs: number;
  endTs: number;
  metadata?: Record<string, unknown>;
  recurrence?: { rule: string };
  timezone?: string;
  localStart?: string;
}) => {
  const body = {
    calendar_uid: calendarUid,
    start_ts: startTs,
    end_ts: endTs,
    ...(accountId && { account_id: accountId }),
    ...(metadata && { metadata }),
    ...(recurrence && { recurrence }),
    ...(timezone && { timezone }),
    ...(localStart && { local_start: localStart }),
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const url = `${process.env.SCHEDULER_URL}/api/v1/events`;

  const response = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });

  return response.json() as Promise<EventObject | ErrorObject>;
};

export const deleteEvent = async ({
  eventUid,
  accountId,
}: {
  eventUid: string;
  accountId?: string;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const url = accountId
    ? `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}?account_id=${accountId}`
    : `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}`;
  const response = await fetch(url, {
    method: "DELETE",
    headers,
  });

  return response.json() as Promise<SuccessObject> | Promise<ErrorObject>;
};

export const getEvent = async ({ eventUid }: { eventUid: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}`,
    {
      method: "GET",
      headers,
    }
  );

  return response.json() as Promise<EventObject | ErrorObject>;
};

export const updateEvent = async ({
  eventUid,
  accountId,
  startTs,
  endTs,
  metadata,
  recurrence,
  scope,
  timezone,
  localStart,
}: {
  eventUid: string;
  accountId: string;
  startTs: number;
  endTs: number;
  metadata?: Record<string, unknown>;
  recurrence?: { rule: string };
  scope?: string;
  timezone?: string;
  localStart?: string;
}) => {
  const body = {
    account_id: accountId,
    start_ts: startTs,
    end_ts: endTs,
    ...(metadata && { metadata }),
    ...(recurrence && { recurrence }),
    ...(timezone && { timezone }),
    ...(localStart && { local_start: localStart }),
    scope,
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}`,
    {
      method: "PUT",
      headers,
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<EventObject | ErrorObject>;
};

export const toggleCancelledStatusEvent = async ({
  eventUid,
}: {
  eventUid: string;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/toggle-cancelled`,
    { method: "POST", headers }
  );
  return response.json() as Promise<EventObject | ErrorObject>;
};

export const getCalendarEvents = async ({
  calendarUids,
  startTs,
  endTs,
}: {
  calendarUids: string | string[];
  startTs: number;
  endTs: number;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
    "Content-Type": "application/json",
  };

  // Support both single calendar UID (string) and multiple (array)
  const calendarUidsArray = Array.isArray(calendarUids)
    ? calendarUids
    : [calendarUids];

  const body = {
    calendar_uids: calendarUidsArray,
    start_ts: startTs,
    end_ts: endTs,
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/events`,
    {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }
  );
  return response.json() as Promise<
    GenericPagedResponse<EventObject> | ErrorObject
  >;
};
