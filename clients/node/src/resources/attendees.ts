import { HttpClient } from "../utils/http";
import {
  Attendee,
  CreateAttendeeRequest,
  UpdateAttendeeRequest,
  UpdateAttendeeRSVPRequest,
} from "../types";

export class Attendees {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async create(
    eventUID: string,
    data: CreateAttendeeRequest
  ): Promise<{ attendee?: Attendee; attendee_group_id?: string; scope: string; count: number }> {
    return this.http.post(`/api/v1/events/${eventUID}/attendees`, data);
  }

  async list(
    eventUID: string,
    roleFilter?: string,
    rsvpFilter?: string
  ): Promise<Attendee[]> {
    const params: Record<string, string> = {};
    if (roleFilter) params.role = roleFilter;
    if (rsvpFilter) params.rsvp_status = rsvpFilter;

    return this.http.get<Attendee[]>(`/api/v1/events/${eventUID}/attendees`, {
      params: Object.keys(params).length > 0 ? params : undefined,
    });
  }

  async get(eventUID: string, accountID: string): Promise<Attendee> {
    return this.http.get<Attendee>(`/api/v1/events/${eventUID}/attendees/${accountID}`);
  }

  async update(
    eventUID: string,
    accountID: string,
    data: UpdateAttendeeRequest
  ): Promise<{ attendee?: Attendee; scope: string; count: number }> {
    return this.http.patch(`/api/v1/events/${eventUID}/attendees/${accountID}`, data);
  }

  async delete(
    eventUID: string,
    accountID: string,
    scope?: "single" | "all"
  ): Promise<{ message: string; scope: string; count: number; reminders_deleted: number }> {
    const params = scope ? { scope } : undefined;
    return this.http.delete(`/api/v1/events/${eventUID}/attendees/${accountID}`, {
      params,
    });
  }

  async updateRSVP(
    eventUID: string,
    accountID: string,
    data: UpdateAttendeeRSVPRequest
  ): Promise<{ attendee?: Attendee; scope: string; count: number }> {
    return this.http.put(`/api/v1/events/${eventUID}/attendees/${accountID}/rsvp`, data);
  }

  async getAccountEvents(
    accountID: string,
    startTs: number,
    endTs: number,
    roleFilter?: string,
    rsvpFilter?: string
  ): Promise<any[]> {
    const params: Record<string, string> = {
      account_id: accountID,
      start_ts: startTs.toString(),
      end_ts: endTs.toString(),
    };
    if (roleFilter) params.role = roleFilter;
    if (rsvpFilter) params.rsvp_status = rsvpFilter;

    return this.http.get<any[]>("/api/v1/attendees/events", {
      params,
    });
  }
}
