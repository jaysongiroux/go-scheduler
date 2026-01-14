import { HttpClient } from "../utils/http";
import {
  Reminder,
  CreateReminderRequest,
  UpdateReminderRequest,
} from "../types";

export class Reminders {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async create(
    eventUID: string,
    data: CreateReminderRequest
  ): Promise<{ reminder: Reminder; scope: string; count: number }> {
    return this.http.post(`/api/v1/events/${eventUID}/reminders`, data);
  }

  async list(eventUID: string, accountID?: string): Promise<Reminder[]> {
    const params = accountID ? { account_id: accountID } : undefined;
    return this.http.get<Reminder[]>(`/api/v1/events/${eventUID}/reminders`, {
      params,
    });
  }

  async update(
    eventUID: string,
    reminderUID: string,
    data: UpdateReminderRequest
  ): Promise<{ reminder: Reminder; scope: string; count: number }> {
    return this.http.put(`/api/v1/events/${eventUID}/reminders/${reminderUID}`, data);
  }

  async delete(
    eventUID: string,
    reminderUID: string,
    scope?: "single" | "all"
  ): Promise<{ message: string; scope: string; count: number }> {
    const params = scope ? { scope } : undefined;
    return this.http.delete(`/api/v1/events/${eventUID}/reminders/${reminderUID}`, {
      params,
    });
  }
}
