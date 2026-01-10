import {
  ErrorObject,
  EventObject,
  GenericPagedResponse,
  SuccessObject,
} from "./types";

export const createEvent = async ({
  calendarUid,
  startTs,
  endTs,
  metadata,
  recurrence,
}: {
  calendarUid: string;
  startTs: number;
  endTs: number;
  metadata?: Record<string, unknown>;
  recurrence?: { rule: string };
}) => {
  const body = {
    calendar_uid: calendarUid,
    start_ts: startTs,
    end_ts: endTs,
    ...(metadata && { metadata }),
    ...(recurrence && { recurrence }),
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(`${process.env.SCHEDULER_URL}/api/v1/events`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });

  return response.json() as Promise<EventObject | ErrorObject>;
};

export const deleteEvent = async ({ eventUid }: { eventUid: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}`,
    {
      method: "DELETE",
      headers,
    }
  );

  return response.json() as Promise<SuccessObject>;
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
  startTs,
  endTs,
  metadata,
  recurrence,
  scope,
}: {
  eventUid: string;
  startTs: number;
  endTs: number;
  metadata?: Record<string, unknown>;
  recurrence?: { rule: string };
  scope?: string;
}) => {
  const body = {
    start_ts: startTs,
    end_ts: endTs,
    ...(metadata && { metadata }),
    ...(recurrence && { recurrence }),
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
  calendarUid,
  startTs,
  endTs,
}: {
  calendarUid: string;
  startTs: number;
  endTs: number;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/${calendarUid}/events?start_ts=${startTs}&end_ts=${endTs}`,
    { method: "GET", headers }
  );
  return response.json() as Promise<
    GenericPagedResponse<EventObject> | ErrorObject
  >;
};
