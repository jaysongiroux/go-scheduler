import { HttpClient } from "../utils/http";
import {
  Event,
  CreateEventRequest,
  UpdateEventRequest,
  GetCalendarEventsRequest,
  PagedResponse,
} from "../types";

export class Events {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async create(data: CreateEventRequest): Promise<Event> {
    return this.http.post<Event>("/api/v1/events", data);
  }

  async get(eventUID: string): Promise<Event> {
    return this.http.get<Event>(`/api/v1/events/${eventUID}`);
  }

  async getCalendarEvents(data: GetCalendarEventsRequest): Promise<PagedResponse<Event>> {
    return this.http.post<PagedResponse<Event>>("/api/v1/calendars/events", data);
  }

  async update(eventUID: string, data: UpdateEventRequest): Promise<Event> {
    return this.http.put<Event>(`/api/v1/events/${eventUID}`, data);
  }

  async delete(eventUID: string, accountID: string, scope?: "single" | "all"): Promise<void> {
    const params: Record<string, string> = { account_id: accountID };
    if (scope) {
      params.scope = scope;
    }

    return this.http.delete<void>(`/api/v1/events/${eventUID}`, {
      params,
    });
  }

  async toggleCancelled(eventUID: string): Promise<Event> {
    return this.http.post<Event>(`/api/v1/events/${eventUID}/toggle-cancelled`);
  }

  async transferOwnership(
    eventUID: string,
    newOrganizerAccountID: string,
    newOrganizerCalendarUID: string,
    scope?: "single" | "all"
  ): Promise<{ message: string; new_organizer: any; scope: string; count: number }> {
    return this.http.post(`/api/v1/events/${eventUID}/transfer-ownership`, {
      new_organizer_account_id: newOrganizerAccountID,
      new_organizer_calendar_uid: newOrganizerCalendarUID,
      scope,
    });
  }
}
