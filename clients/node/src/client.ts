import { HttpClient } from "./utils/http";
import { Calendars } from "./resources/calendars";
import { Events } from "./resources/events";
import { Reminders } from "./resources/reminders";
import { Webhooks } from "./resources/webhooks";
import { Attendees } from "./resources/attendees";
import { CalendarMembers } from "./resources/calendarMembers";
import { SchedulerConfig } from "./types";

export class SchedulerClient {
  private http: HttpClient;

  public calendars: Calendars;
  public events: Events;
  public reminders: Reminders;
  public webhooks: Webhooks;
  public attendees: Attendees;
  public calendarMembers: CalendarMembers;

  constructor(config: SchedulerConfig) {
    this.http = new HttpClient(config.baseURL, config.timeout, config.headers);

    this.calendars = new Calendars(this.http);
    this.events = new Events(this.http);
    this.reminders = new Reminders(this.http);
    this.webhooks = new Webhooks(this.http);
    this.attendees = new Attendees(this.http);
    this.calendarMembers = new CalendarMembers(this.http);
  }

  setHeader(key: string, value: string): void {
    this.http.setHeader(key, value);
  }

  removeHeader(key: string): void {
    this.http.removeHeader(key);
  }

  async healthCheck(): Promise<{ status: string }> {
    return this.http.get<{ status: string }>("/health");
  }
}
