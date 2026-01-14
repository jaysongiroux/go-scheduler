import { HttpClient } from "../utils/http";
import {
  CalendarMember,
  InviteCalendarMembersRequest,
  UpdateCalendarMemberRequest,
} from "../types";

export class CalendarMembers {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async invite(
    calendarUID: string,
    data: InviteCalendarMembersRequest
  ): Promise<{ invited_count: number; members: CalendarMember[] }> {
    return this.http.post(`/api/v1/calendars/${calendarUID}/members`, data);
  }

  async list(calendarUID: string, requestingAccountID: string): Promise<CalendarMember[]> {
    return this.http.get<CalendarMember[]>(`/api/v1/calendars/${calendarUID}/members`, {
      params: { account_id: requestingAccountID },
    });
  }

  async update(
    calendarUID: string,
    memberAccountID: string,
    requestingAccountID: string,
    data: UpdateCalendarMemberRequest
  ): Promise<CalendarMember> {
    return this.http.put<CalendarMember>(
      `/api/v1/calendars/${calendarUID}/members/${memberAccountID}`,
      data,
      {
        params: { account_id: requestingAccountID },
      }
    );
  }

  async remove(calendarUID: string, memberAccountID: string, requestingAccountID: string): Promise<void> {
    return this.http.delete<void>(`/api/v1/calendars/${calendarUID}/members/${memberAccountID}`, {
      params: { account_id: requestingAccountID },
    });
  }
}
