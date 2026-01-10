import {
  CalendarObject,
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
} from "./types";

export const createCalendar = async ({
  accountId,
  settings,
  metadata,
}: {
  accountId: string;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}) => {
  const body = {
    account_id: accountId,
    ...(settings && { settings }),
    ...(metadata && { metadata }),
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars`,
    {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<CalendarObject | ErrorObject>;
};

export const deleteCalendar = async ({
  calendarUid,
}: {
  calendarUid: string;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/${calendarUid}`,
    {
      method: "DELETE",
      headers,
    }
  );

  return response.json() as Promise<SuccessObject | ErrorObject>;
};

export const getCalendar = async ({ calendarUid }: { calendarUid: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/${calendarUid}`,
    { method: "GET", headers }
  );
  return response.json() as Promise<CalendarObject | ErrorObject>;
};

export const updateCalendar = async ({
  calendarUid,
  settings,
  metadata,
}: {
  calendarUid: string;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}) => {
  const body = {
    ...(settings && { settings }),
    ...(metadata && { metadata }),
  };

  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/${calendarUid}`,
    { method: "PUT", headers, body: JSON.stringify(body) }
  );
  return response.json() as Promise<CalendarObject | ErrorObject>;
};

export const getUserCalendars = async ({
  accountId,
}: {
  accountId: string;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/account/${accountId}/calendars`,
    { method: "GET", headers }
  );
  return response.json() as Promise<
    GenericPagedResponse<CalendarObject> | ErrorObject
  >;
};
