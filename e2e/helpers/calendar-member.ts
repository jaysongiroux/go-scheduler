import { CalendarMemberObject, ErrorObject, SuccessObject } from "./types";

const API_BASE_URL = process.env.SCHEDULER_URL || "";

const getHeaders = () => ({
  "api-key": process.env.SCHEDULER_API_KEY || "",
  "Content-Type": "application/json",
});

export async function inviteCalendarMembers(params: {
  calendarUid: string;
  accountId: string;
  accountIds: string[];
  role?: string;
}): Promise<
  { invited_count: number; members: CalendarMemberObject[] } | ErrorObject
> {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/calendars/${params.calendarUid}/members`,
    {
      method: "POST",
      headers: getHeaders(),
      body: JSON.stringify({
        account_id: params.accountId,
        account_ids: params.accountIds,
        role: params.role || "write",
      }),
    }
  );
  return await response.json();
}

export async function getCalendarMembers(params: {
  calendarUid: string;
  accountId: string;
}): Promise<CalendarMemberObject[] | ErrorObject> {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/calendars/${params.calendarUid}/members?account_id=${params.accountId}`,
    {
      method: "GET",
      headers: getHeaders(),
    }
  );
  return await response.json();
}

export async function updateCalendarMember(params: {
  calendarUid: string;
  memberAccountId: string;
  requestingAccountId: string;
  status?: string;
  role?: string;
}): Promise<CalendarMemberObject | SuccessObject | ErrorObject> {
  const body: { status?: string; role?: string } = {};
  if (params.status !== undefined) {
    body.status = params.status;
  }
  if (params.role !== undefined) {
    body.role = params.role;
  }

  const response = await fetch(
    `${API_BASE_URL}/api/v1/calendars/${params.calendarUid}/members/${params.memberAccountId}?account_id=${params.requestingAccountId}`,
    {
      method: "PUT",
      headers: getHeaders(),
      body: JSON.stringify(body),
    }
  );
  return await response.json();
}

export async function removeCalendarMember(params: {
  calendarUid: string;
  memberAccountId: string;
  requestingAccountId: string;
}): Promise<SuccessObject | ErrorObject> {
  const response = await fetch(
    `${API_BASE_URL}/api/v1/calendars/${params.calendarUid}/members/${params.memberAccountId}?account_id=${params.requestingAccountId}`,
    {
      method: "DELETE",
      headers: getHeaders(),
    }
  );
  return await response.json();
}
