import {
  CalendarObject,
  ErrorObject,
  GenericPagedResponse,
  ICSImportResponse,
  ICSLinkImportResponse,
  ResyncResponse,
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
    `${process.env.SCHEDULER_URL}/api/v1/calendars?account_id=${accountId}`,
    { method: "GET", headers }
  );
  return response.json() as Promise<
    GenericPagedResponse<CalendarObject> | ErrorObject
  >;
};

export const importICS = async ({
  file,
  accountId,
  calendarUid,
  calendarMetadata,
  importReminders = true,
  importAttendees = true,
}: {
  file: Buffer | Blob;
  accountId: string;
  calendarUid?: string;
  calendarMetadata?: Record<string, unknown>;
  importReminders?: boolean;
  importAttendees?: boolean;
}) => {
  const headers: Record<string, string> = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };

  const formData = new FormData();
  const blob = file instanceof Blob ? file : new Blob([new Uint8Array(file)]);
  formData.append("file", blob, "calendar.ics");
  formData.append("account_id", accountId);

  if (calendarUid) {
    formData.append("calendar_uid", calendarUid);
  }

  if (calendarMetadata) {
    formData.append("calendar_metadata", JSON.stringify(calendarMetadata));
  }

  formData.append("import_reminders", importReminders.toString());
  formData.append("import_attendees", importAttendees.toString());

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/import/ics`,
    {
      method: "POST",
      headers,
      body: formData,
    }
  );

  return response.json() as Promise<ICSImportResponse | ErrorObject>;
};

export const importICSLink = async ({
  accountId,
  icsUrl,
  authType,
  authCredentials,
  syncIntervalSeconds,
  calendarMetadata,
  syncOnPartialFailure = true,
}: {
  accountId: string;
  icsUrl: string;
  authType: "none" | "basic" | "bearer";
  authCredentials?: string;
  syncIntervalSeconds?: number;
  calendarMetadata?: Record<string, unknown>;
  syncOnPartialFailure?: boolean;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
    "Content-Type": "application/json",
  };

  const body = {
    account_id: accountId,
    ics_url: icsUrl,
    auth_type: authType,
    ...(authCredentials && { auth_credentials: authCredentials }),
    ...(syncIntervalSeconds && { sync_interval_seconds: syncIntervalSeconds }),
    ...(calendarMetadata && { calendar_metadata: calendarMetadata }),
    sync_on_partial_failure: syncOnPartialFailure,
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/import/ics-link`,
    {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<ICSLinkImportResponse | ErrorObject>;
};

export const resyncCalendar = async ({
  calendarUid,
}: {
  calendarUid: string;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/calendars/${calendarUid}/resync`,
    {
      method: "POST",
      headers,
    }
  );

  return response.json() as Promise<ResyncResponse | ErrorObject>;
};
