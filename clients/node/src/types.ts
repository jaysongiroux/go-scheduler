export interface SchedulerConfig {
  baseURL: string;
  timeout?: number;
  headers?: Record<string, string>;
}

export interface Calendar {
  calendar_uid: string;
  account_id: string;
  name?: string;
  description?: string;
  timezone?: string;
  metadata?: Record<string, any>;
  created_ts: number;
  updated_ts: number;
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
}

export interface CreateCalendarRequest {
  account_id: string;
  name?: string;
  description?: string;
  timezone?: string;
  metadata?: Record<string, any>;
}

export interface UpdateCalendarRequest {
  name?: string;
  description?: string;
  timezone?: string;
  metadata?: Record<string, any>;
}

export interface Recurrence {
  rule: string;
  until?: number;
  count?: number;
}

export interface Event {
  event_uid: string;
  calendar_uid: string;
  account_id: string;
  start_ts: number;
  end_ts: number;
  duration: number;
  timezone?: string;
  local_start?: string;
  metadata?: Record<string, any>;
  recurrence?: Recurrence;
  recurrence_status?: string;
  recurrence_end_ts?: number;
  is_recurring_instance?: boolean;
  is_modified?: boolean;
  master_event_uid?: string;
  original_start_ts?: number;
  exdates_ts?: number[];
  is_cancelled?: boolean;
  created_ts: number;
  updated_ts: number;
}

export interface CreateEventRequest {
  calendar_uid: string;
  account_id: string;
  start_ts: number;
  end_ts: number;
  timezone?: string;
  local_start?: string;
  metadata?: Record<string, any>;
  recurrence?: Recurrence;
}

export type UpdateScope = "single" | "future" | "all";

export interface UpdateEventRequest {
  account_id: string;
  start_ts: number;
  end_ts: number;
  timezone?: string;
  local_start?: string;
  metadata?: Record<string, any>;
  recurrence?: Recurrence;
  scope?: UpdateScope;
}

export interface GetCalendarEventsRequest {
  calendar_uids: string[];
  start_ts: number;
  end_ts: number;
}

export interface Reminder {
  reminder_uid?: string;
  reminder_group_id?: string;
  event_uid: string;
  account_id: string;
  offset_seconds: number;
  trigger_ts?: number;
  metadata?: Record<string, any>;
  is_delivered?: boolean;
  delivered_ts?: number;
  is_archived?: boolean;
  created_ts?: number;
  updated_ts?: number;
}

export interface CreateReminderRequest {
  offset_seconds: number;
  account_id: string;
  metadata?: Record<string, any>;
  scope?: "single" | "all";
}

export interface UpdateReminderRequest {
  offset_seconds: number;
  metadata?: Record<string, any>;
  scope?: "single" | "all";
}

export interface Webhook {
  webhook_uid: string;
  url: string;
  event_types?: string[];
  secret: string;
  retry_count: number;
  timeout_seconds: number;
  failure_count?: number;
  is_active?: boolean;
  created_ts: number;
  updated_ts: number;
}

export interface CreateWebhookRequest {
  url: string;
  event_types?: string[];
  secret?: string;
  retry_count?: number;
  timeout_seconds?: number;
}

export interface UpdateWebhookRequest {
  url?: string;
  event_types?: string[];
  retry_count?: number;
  timeout_seconds?: number;
  is_active?: boolean;
}

export interface WebhookDelivery {
  delivery_uid: string;
  webhook_uid: string;
  event_type: string;
  payload: Record<string, any>;
  status_code?: number;
  response_body?: string;
  error_message?: string;
  attempt_count: number;
  created_ts: number;
  delivered_ts?: number;
}

export interface Attendee {
  attendee_uid: string;
  event_uid: string;
  account_id: string;
  role: "organizer" | "attendee";
  rsvp_status: "pending" | "accepted" | "declined" | "tentative";
  attendee_group_id?: string;
  metadata?: Record<string, any>;
  created_ts: number;
  updated_ts: number;
}

export interface CreateAttendeeRequest {
  account_id: string;
  role?: "organizer" | "attendee";
  metadata?: Record<string, any>;
  scope?: "single" | "all";
}

export interface UpdateAttendeeRequest {
  role?: "organizer" | "attendee";
  metadata?: Record<string, any>;
  scope?: "single" | "all";
}

export interface UpdateAttendeeRSVPRequest {
  rsvp_status: "pending" | "accepted" | "declined" | "tentative";
  scope?: "single" | "all";
}

export interface TransferOwnershipRequest {
  new_organizer_account_id: string;
  new_organizer_calendar_uid: string;
  scope?: "single" | "all";
}

export interface CalendarMember {
  calendar_uid: string;
  account_id: string;
  role: "read" | "write";
  status: "pending" | "confirmed";
  created_ts: number;
  updated_ts: number;
}

export interface InviteCalendarMembersRequest {
  account_id: string; // The inviter's account ID
  account_ids: string[]; // Array of account IDs to invite
  role: "read" | "write"; // Role for all invitees
}

export interface UpdateCalendarMemberRequest {
  role?: "read" | "write";
  status?: "pending" | "confirmed";
}

export interface PagedResponse<T> {
  data: T[];
  total: number;
  limit?: number;
  offset?: number;
}

export interface ErrorResponse {
  error: string;
  details?: string;
}

// ICS Import Types
export interface ICSImportOptions {
  accountId: string;
  calendarUid?: string;
  calendarMetadata?: Record<string, any>;
  importReminders?: boolean;
  importAttendees?: boolean;
}

export interface ICSImportSummary {
  total_events: number;
  imported_events: number;
  failed_events: number;
}

export interface ICSImportEventResult {
  ics_uid: string;
  event_uid?: string;
  status: "success" | "failed";
  error?: string;
}

export interface ICSImportResponse {
  calendar: Calendar;
  summary: ICSImportSummary;
  events: ICSImportEventResult[];
}

export interface ICSLinkImportOptions {
  accountId: string;
  icsUrl: string;
  authType: "none" | "basic" | "bearer";
  authCredentials?: string;
  syncIntervalSeconds?: number;
  calendarMetadata?: Record<string, any>;
  syncOnPartialFailure?: boolean;
}

export interface ICSLinkImportResponse {
  calendar: Calendar;
  summary: ICSImportSummary;
  events: ICSImportEventResult[];
  sync_scheduled: boolean;
}

export interface ResyncResponse {
  calendar: Calendar;
  imported_events: number;
  failed_events: number;
  warnings: string[];
}

// Webhook Event Types
export type WebhookEventType =
  // Event webhooks
  | "event.created"
  | "event.updated"
  | "event.deleted"
  | "event.cancelled"
  | "event.uncancelled"
  | "event.ownership_transferred"
  // Calendar webhooks
  | "calendar.created"
  | "calendar.updated"
  | "calendar.deleted"
  | "calendar.synced"
  | "calendar.resynced"
  // Calendar member webhooks
  | "member.invited"
  | "member.accepted"
  | "member.rejected"
  | "member.removed"
  | "member.status_updated"
  | "member.role_updated"
  // Reminder webhooks
  | "reminder.created"
  | "reminder.updated"
  | "reminder.deleted"
  | "reminder.triggered"
  | "reminder.due"
  // Attendee webhooks
  | "attendee.created"
  | "attendee.updated"
  | "attendee.deleted"
  | "attendee.rsvp_updated";

// Webhook Payload Structure
export interface WebhookPayload<T = any> {
  webhook_uid: string;
  event_type: WebhookEventType;
  delivery_id: string;
  timestamp: number;
  data: T;
}

// Specific webhook payload data types
export interface CalendarWebhookData {
  calendar_uid: string;
  account_id: string;
  created_ts?: number;
  updated_ts?: number;
  metadata?: Record<string, any>;
}

export interface CalendarSyncedWebhookData {
  calendar_uid: string;
  account_id: string;
  imported_events: number;
  failed_events: number;
  warnings: string[];
  sync_ts: number;
  manual_trigger: boolean;
}

export interface EventWebhookData {
  event_uid: string;
  calendar_uid: string;
  account_id: string;
  start_ts: number;
  end_ts?: number;
  metadata?: Record<string, any>;
}

export interface EventBatchWebhookData {
  events: EventWebhookData[];
  count: number;
}

export interface ReminderWebhookData {
  reminder_uid: string;
  event_uid: string;
  account_id: string;
  offset_seconds: number;
  metadata?: Record<string, any>;
}

export interface ReminderDueWebhookData {
  reminder_uid: string;
  event_uid: string;
  account_id: string;
  offset_seconds: number;
  remind_at_ts: number;
  reminder_metadata?: Record<string, any>;
  event: {
    event_uid: string;
    calendar_uid: string;
    start_ts: number;
    event_metadata?: Record<string, any>;
  };
}

export interface AttendeeWebhookData {
  event_uid: string;
  account_id: string;
  role: "organizer" | "attendee";
  rsvp_status: "pending" | "accepted" | "declined" | "tentative";
  scope?: "single" | "all";
  count?: number;
}

export interface MemberWebhookData {
  calendar_uid: string;
  account_id: string;
  role: "read" | "write";
  status: "pending" | "confirmed";
  invited_by: string;
}

// Webhook verification result
export interface WebhookVerificationResult {
  valid: boolean;
  error?: string;
}

// Type guards for webhook payloads
export function isCalendarWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<CalendarWebhookData> {
  return (
    payload.event_type.startsWith("calendar.") &&
    payload.event_type !== "calendar.synced" &&
    payload.event_type !== "calendar.resynced"
  );
}

export function isCalendarSyncedWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<CalendarSyncedWebhookData> {
  return (
    payload.event_type === "calendar.synced" ||
    payload.event_type === "calendar.resynced"
  );
}

export function isEventWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<EventWebhookData | EventWebhookData[]> {
  return payload.event_type.startsWith("event.");
}

export function isReminderDueWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<ReminderDueWebhookData> {
  return (
    payload.event_type === "reminder.due" ||
    payload.event_type === "reminder.triggered"
  );
}

export function isAttendeeWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<AttendeeWebhookData> {
  return payload.event_type.startsWith("attendee.");
}

export function isMemberWebhook(
  payload: WebhookPayload
): payload is WebhookPayload<MemberWebhookData> {
  return payload.event_type.startsWith("member.");
}
