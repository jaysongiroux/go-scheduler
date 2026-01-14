import { AttendeeObject, ErrorObject } from "./types";

export const createAttendee = async ({
  eventUid,
  accountId,
  role,
  scope,
  metadata,
}: {
  eventUid: string;
  accountId: string;
  role?: "organizer" | "attendee";
  scope?: "single" | "all";
  metadata?: Record<string, unknown>;
}) => {
  const body = {
    account_id: accountId,
    ...(role && { role }),
    ...(scope && { scope }),
    ...(metadata && { metadata }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees`,
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
    | { attendee: AttendeeObject; scope: string; count: number }
    | { attendee_group_id: string; scope: string; count: number }
    | ErrorObject
  >;
};

export const getEventAttendees = async ({
  eventUid,
  role,
  rsvpStatus,
}: {
  eventUid: string;
  role?: "organizer" | "attendee";
  rsvpStatus?: "pending" | "accepted" | "declined" | "tentative";
}) => {
  let url = `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees`;
  const params = new URLSearchParams();
  if (role) params.append("role", role);
  if (rsvpStatus) params.append("rsvp_status", rsvpStatus);
  if (params.toString()) url += `?${params.toString()}`;

  const response = await fetch(url, {
    method: "GET",
    headers: {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
  });

  return response.json() as Promise<AttendeeObject[] | ErrorObject>;
};

export const getAttendee = async ({
  eventUid,
  accountId,
}: {
  eventUid: string;
  accountId: string;
}) => {
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees/${accountId}`,
    {
      method: "GET",
      headers: {
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
    }
  );

  return response.json() as Promise<AttendeeObject | ErrorObject>;
};

export const updateAttendee = async ({
  eventUid,
  accountId,
  role,
  metadata,
  scope,
}: {
  eventUid: string;
  accountId: string;
  role?: "organizer" | "attendee";
  metadata?: Record<string, unknown>;
  scope?: "single" | "all";
}) => {
  const body = {
    ...(role && { role }),
    ...(metadata && { metadata }),
    ...(scope && { scope }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees/${accountId}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<
    | { attendee: AttendeeObject; scope: string; count: number }
    | { scope: string; count: number }
    | ErrorObject
  >;
};

export const deleteAttendee = async ({
  eventUid,
  accountId,
  scope,
}: {
  eventUid: string;
  accountId: string;
  scope?: "single" | "all";
}) => {
  let url = `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees/${accountId}`;
  if (scope) url += `?scope=${scope}`;

  const response = await fetch(url, {
    method: "DELETE",
    headers: {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
  });

  return response.json() as Promise<
    { message: string; scope: string; count: number; reminders_deleted: number }
    | ErrorObject
  >;
};

export const updateAttendeeRSVP = async ({
  eventUid,
  accountId,
  rsvpStatus,
  scope,
}: {
  eventUid: string;
  accountId: string;
  rsvpStatus: "pending" | "accepted" | "declined" | "tentative";
  scope?: "single" | "all";
}) => {
  const body = {
    rsvp_status: rsvpStatus,
    ...(scope && { scope }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/attendees/${accountId}/rsvp`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "api-key": process.env.SCHEDULER_API_KEY || "",
      },
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<
    | { attendee: AttendeeObject; scope: string; count: number }
    | { scope: string; count: number }
    | ErrorObject
  >;
};

export const transferOwnership = async ({
  eventUid,
  newOrganizerAccountId,
  newOrganizerCalendarUid,
  scope,
}: {
  eventUid: string;
  newOrganizerAccountId: string;
  newOrganizerCalendarUid: string;
  scope?: "single" | "all";
}) => {
  const body = {
    new_organizer_account_id: newOrganizerAccountId,
    new_organizer_calendar_uid: newOrganizerCalendarUid,
    ...(scope && { scope }),
  };

  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/events/${eventUid}/transfer-ownership`,
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
    | {
        message: string;
        new_organizer: { account_id: string; calendar_uid: string };
        scope: string;
        count: number;
      }
    | ErrorObject
  >;
};

export const getAttendeeEvents = async ({
  accountId,
  startTs,
  endTs,
  role,
  rsvpStatus,
}: {
  accountId: string;
  startTs: number;
  endTs: number;
  role?: "organizer" | "attendee";
  rsvpStatus?: "pending" | "accepted" | "declined" | "tentative";
}) => {
  let url = `${process.env.SCHEDULER_URL}/api/v1/attendees/events`;
  const params = new URLSearchParams();
  params.append("account_id", accountId);
  params.append("start_ts", startTs.toString());
  params.append("end_ts", endTs.toString());
  if (role) params.append("role", role);
  if (rsvpStatus) params.append("rsvp_status", rsvpStatus);
  url += `?${params.toString()}`;

  const response = await fetch(url, {
    method: "GET",
    headers: {
      "api-key": process.env.SCHEDULER_API_KEY || "",
    },
  });

  return response.json() as Promise<
    | Array<{
        event: {
          event_uid: string;
          calendar_uid: string;
          start_ts: number;
          end_ts: number;
          metadata: Record<string, unknown>;
          is_cancelled: boolean;
          master_event_uid?: string | null;
        };
        attendee: {
          attendee_uid: string;
          role: string;
          rsvp_status: string;
        };
      }>
    | ErrorObject
  >;
};
