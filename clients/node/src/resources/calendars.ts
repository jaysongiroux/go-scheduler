import { HttpClient } from "../utils/http";
import {
  Calendar,
  CreateCalendarRequest,
  UpdateCalendarRequest,
  PagedResponse,
  ICSImportOptions,
  ICSImportResponse,
  ICSLinkImportOptions,
  ICSLinkImportResponse,
  ResyncResponse,
} from "../types";

export class Calendars {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async create(data: CreateCalendarRequest): Promise<Calendar> {
    return this.http.post<Calendar>("/api/v1/calendars", data);
  }

  async get(calendarUID: string): Promise<Calendar> {
    return this.http.get<Calendar>(`/api/v1/calendars/${calendarUID}`);
  }

  async list(
    accountID: string,
    limit: number = 50,
    offset: number = 0
  ): Promise<PagedResponse<Calendar>> {
    return this.http.get<PagedResponse<Calendar>>("/api/v1/calendars", {
      params: {
        account_id: accountID,
        limit: limit.toString(),
        offset: offset.toString(),
      },
    });
  }

  async update(
    calendarUID: string,
    data: UpdateCalendarRequest
  ): Promise<Calendar> {
    return this.http.put<Calendar>(`/api/v1/calendars/${calendarUID}`, data);
  }

  async delete(calendarUID: string): Promise<void> {
    return this.http.delete<void>(`/api/v1/calendars/${calendarUID}`);
  }

  /**
   * Import events from an ICS file
   * @param file - The ICS file to import (Buffer or Blob)
   * @param options - Import options
   * @returns Import response with summary and event results
   */
  async importICS(
    file: Buffer | Blob,
    options: ICSImportOptions
  ): Promise<ICSImportResponse> {
    const formData = new FormData();

    // Add file
    if (Buffer.isBuffer(file)) {
      formData.append("file", new Blob([file]), "calendar.ics");
    } else {
      formData.append("file", file, "calendar.ics");
    }

    // Add required fields
    formData.append("account_id", options.accountId);

    // Add optional fields
    if (options.calendarUid) {
      formData.append("calendar_uid", options.calendarUid);
    }

    if (options.calendarMetadata) {
      formData.append(
        "calendar_metadata",
        JSON.stringify(options.calendarMetadata)
      );
    }

    if (options.importReminders !== undefined) {
      formData.append("import_reminders", options.importReminders.toString());
    }

    if (options.importAttendees !== undefined) {
      formData.append("import_attendees", options.importAttendees.toString());
    }

    return this.http.postForm<ICSImportResponse>(
      "/api/v1/calendars/import/ics",
      formData
    );
  }

  /**
   * Import events from an ICS URL
   * @param options - Import options including URL and authentication
   * @returns Import response with summary and event results
   */
  async importICSLink(
    options: ICSLinkImportOptions
  ): Promise<ICSLinkImportResponse> {
    const body = {
      account_id: options.accountId,
      ics_url: options.icsUrl,
      auth_type: options.authType,
      ...(options.authCredentials && {
        auth_credentials: options.authCredentials,
      }),
      ...(options.syncIntervalSeconds && {
        sync_interval_seconds: options.syncIntervalSeconds,
      }),
      ...(options.calendarMetadata && {
        calendar_metadata: options.calendarMetadata,
      }),
      ...(options.syncOnPartialFailure !== undefined && {
        sync_on_partial_failure: options.syncOnPartialFailure,
      }),
    };

    return this.http.post<ICSLinkImportResponse>(
      "/api/v1/calendars/import/ics-link",
      body
    );
  }

  /**
   * Manually resync an ICS calendar
   * @param calendarUID - The calendar UID to resync
   * @returns Resync response with summary
   */
  async resync(calendarUID: string): Promise<ResyncResponse> {
    return this.http.post<ResyncResponse>(
      `/api/v1/calendars/${calendarUID}/resync`,
      {}
    );
  }
}
