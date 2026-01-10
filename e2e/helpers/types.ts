export type EventObject = {
  event_uid: string;
  calendar_uid: string;
  account_id: string;
  start_ts: number;
  duration: number;
  end_ts: number;
  created_ts: number;
  updated_ts: number;

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

export type AccountObject = {
  account_id: string;
  created_ts: number;
  updated_ts: number;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
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
