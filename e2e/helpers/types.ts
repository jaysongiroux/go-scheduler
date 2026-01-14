export type EventObject = {
  event_uid: string;
  calendar_uid: string;
  account_id: string;
  start_ts: number;
  duration: number;
  end_ts: number;
  created_ts: number;
  updated_ts: number;

  // Timezone support for DST-aware scheduling
  timezone?: string | null;
  local_start?: string | null;

  // Recurrence tracking
  recurrence?: { rule: string } | null;
  recurrence_status?: "active" | "inactive" | null;
  recurrence_end_ts?: number | null;
  exdates_ts?: number[];

  // Instance tracking
  is_recurring_instance: boolean;
  master_event_uid?: string | null;
  original_start_ts?: number | null;

  // State tracking
  is_modified: boolean;
  is_cancelled: boolean;

  // Metadata and reminders
  metadata: Record<string, unknown>;
  reminders?: ReminderObject[];
};

export type ErrorObject = {
  error: string;
  details: string;
};

export type SuccessObject = {
  success: boolean;
};

export type GenericPagedResponse<T> = {
  count: number;
  data: T[];
  limit: number;
  offset: number;
};

export type CalendarObject = {
  calendar_uid: string;
  account_id: string;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  is_owner?: boolean;
  members?: CalendarMemberObject[];
  // ICS import fields
  ics_url?: string | null;
  ics_auth_type?: string | null;
  ics_last_sync_ts?: number | null;
  ics_last_sync_status?: string | null;
  ics_sync_interval_seconds?: number | null;
  ics_error_message?: string | null;
  ics_last_etag?: string | null;
  ics_last_modified?: string | null;
  is_read_only?: boolean;
  sync_on_partial_failure?: boolean;
};

export type CalendarMemberObject = {
  account_id: string;
  calendar_uid: string;
  status: "pending" | "confirmed";
  role: "read" | "write";
  invited_by: string;
  invited_at_ts: number;
  updated_ts: number;
};

export type WebhookObject = {
  webhook_uid: string;
  url: string;
  event_types: string[];
  is_active: boolean;
  secret: string;
  retry_count: number;
  timeout_seconds: number;
  last_triggered_at_ts: number;
  last_success_at_ts: number;
  last_failure_at_ts: number;
  failure_count: number;
};

export type WebhookDeliveryObject = {
  delivery_uid: string;
  webhook_uid: string;
  event_type: string;
  payload: { data: Record<string, unknown> };
  http_status: number;
  error_message: string;
  response_body: string;
  response_time_ms: number;
  attempt_number: number;
  delivered_at_ts: number;
};

export type ReminderObject = {
  reminder_uid: string;
  event_uid: string;
  account_id: string;
  master_event_uid?: string | null;
  reminder_group_id?: string | null;
  offset_seconds: number;
  metadata: Record<string, unknown>;
  is_delivered: boolean;
  delivered_ts?: number | null;
  created_ts: number;
  archived: boolean;
  archived_ts?: number | null;
};

export type AttendeeObject = {
  attendee_uid: string;
  event_uid: string;
  account_id: string;
  master_event_uid?: string | null;
  attendee_group_id?: string | null;
  role: "organizer" | "attendee";
  rsvp_status: "pending" | "accepted" | "declined" | "tentative";
  metadata: Record<string, unknown>;
  created_ts: number;
  updated_ts: number;
  archived: boolean;
  archived_ts?: number | null;
};

// ICS Import Types
export type ICSImportSummary = {
  total_events: number;
  imported_events: number;
  failed_events: number;
};

export type ICSImportEventResult = {
  ics_uid: string;
  event_uid?: string;
  status: "success" | "failed";
  error?: string;
};

export type ICSImportResponse = {
  calendar: CalendarObject;
  summary: ICSImportSummary;
  events: ICSImportEventResult[];
};

export type ICSLinkImportResponse = {
  calendar: CalendarObject;
  summary: ICSImportSummary;
  events: ICSImportEventResult[];
  sync_scheduled: boolean;
};

export type ResyncResponse = {
  calendar: CalendarObject;
  imported_events: number;
  failed_events: number;
  warnings: string[];
};
