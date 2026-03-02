package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/attendee"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar_member"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/webhook"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// CalendarRepository defines the interface for calendar operations
type CalendarRepository interface {
	CreateCalendar(ctx context.Context, cal *calendar.Calendar) error
	GetCalendar(ctx context.Context, calendarUID uuid.UUID) (*calendar.Calendar, error)
	GetUserCalendars(
		ctx context.Context,
		userID string,
		limit int,
		offset int,
	) ([]*calendar.Calendar, error)
	UpdateCalendar(ctx context.Context, cal *calendar.Calendar) error
	DeleteCalendar(ctx context.Context, calendarUID uuid.UUID) error
	GetCalendarsNeedingSync(ctx context.Context, batchSize int) ([]*calendar.Calendar, error)
	UpdateSyncStatus(
		ctx context.Context,
		calendarUID uuid.UUID,
		status string,
		errorMsg *string,
		timestamp int64,
		etag *string,
		lastModified *string,
	) error
}

// WebhookRepository defines the interface for webhook operations
type WebhookRepository interface {
	CreateWebhook(ctx context.Context, w *webhook.Webhook) error
	GetWebhook(ctx context.Context, webhookUID uuid.UUID) (*webhook.Webhook, error)
	GetWebhookEndpoints(ctx context.Context, offset, limit int) ([]*webhook.Webhook, error)
	UpdateWebhook(ctx context.Context, w *webhook.Webhook) error
	DeleteWebhook(ctx context.Context, webhookUID uuid.UUID) error
	GetWebhookDeliveries(
		ctx context.Context,
		webhookUID uuid.UUID,
		limit int,
		offset int,
	) ([]*webhook.WebhookDelivery, error)
}

// EventRepository defines the interface for event operations
type EventRepository interface {
	CreateEvent(ctx context.Context, evt *event.Event) (event.Event, error)
	GetEvent(
		ctx context.Context,
		eventUID uuid.UUID,
		includeArchivedReminders *bool,
	) (*event.Event, error)
	GetCalendarEvents(
		ctx context.Context,
		calendarUIDs []uuid.UUID,
		startTs, endTs int64,
	) ([]*event.Event, error)
	UpdateEvent(ctx context.Context, evt *event.Event) error
	DeleteEvent(ctx context.Context, eventUID uuid.UUID) error

	// Recurring event operations
	CreateEventWithInstances(
		ctx context.Context,
		evt *event.Event,
		window event.GenerationWindow,
	) (*event.Event, []*event.Event, error)
	DeleteFutureInstances(ctx context.Context, masterUID uuid.UUID, afterTs int64) error
	GetFutureInstances(
		ctx context.Context,
		masterUID uuid.UUID,
		fromTs int64,
	) ([]*event.Event, error)
	AddExDate(ctx context.Context, masterUID uuid.UUID, exdateTs int64) error
	CancelInstance(ctx context.Context, instanceUID uuid.UUID) error
	ToggleCancelledStatusEvent(ctx context.Context, eventUID uuid.UUID) error
	CountInstancesByMaster(ctx context.Context, masterUID uuid.UUID) (int, error)
	GetInstanceByMasterAndOriginalStart(ctx context.Context, masterUID uuid.UUID, originalStartTs int64) (*event.Event, error)
	InsertSingleInstance(ctx context.Context, inst *event.Event) error
}

// ReminderRepository defines the interface for reminder operations
type ReminderRepository interface {
	CreateSingleReminder(ctx context.Context, reminder *event.Reminder) (*event.Reminder, error)
	CreateSeriesReminder(ctx context.Context, reminder *event.Reminder) (uuid.UUID, int, error)
	UpdateSingleReminder(ctx context.Context, reminder *event.Reminder) error
	UpdateSeriesReminder(ctx context.Context, reminder *event.Reminder) (int, error)
	DeleteSingleReminder(ctx context.Context, reminderUID uuid.UUID) error
	DeleteSeriesReminder(ctx context.Context, reminderUID uuid.UUID) (int, error)
	GetEventReminders(
		ctx context.Context,
		eventUID uuid.UUID,
		accountID *string,
	) ([]*event.Reminder, error)
	CopyRemindersToNewEvent(ctx context.Context, masterEventUID, newEventUID uuid.UUID) (int, error)
	GetReminderByUID(ctx context.Context, reminderUID uuid.UUID) (*event.Reminder, error)
	MarkReminderDelivered(ctx context.Context, reminderUID uuid.UUID) error
	GetDueReminders(ctx context.Context, beforeTs int64, limit int) ([]*event.Reminder, error)
	GetDueRemindersWithEvents(
		ctx context.Context,
		beforeTs int64,
		limit int,
	) ([]*event.ReminderWithEvent, error)
}

// CalendarMemberRepository defines the interface for calendar member operations
type CalendarMemberRepository interface {
	CreateCalendarMember(ctx context.Context, member *calendar_member.CalendarMember) error
	CreateCalendarMembers(ctx context.Context, members []*calendar_member.CalendarMember) error
	GetCalendarMembers(
		ctx context.Context,
		calendarUID uuid.UUID,
	) ([]*calendar_member.CalendarMember, error)
	GetMemberCalendars(
		ctx context.Context,
		accountID string,
		limit int,
		offset int,
	) ([]uuid.UUID, error)
	GetCalendarMember(
		ctx context.Context,
		accountID string,
		calendarUID uuid.UUID,
	) (*calendar_member.CalendarMember, error)
	UpdateMemberStatus(
		ctx context.Context,
		accountID string,
		calendarUID uuid.UUID,
		status string,
		updatedTs int64,
	) error
	UpdateMemberRole(
		ctx context.Context,
		accountID string,
		calendarUID uuid.UUID,
		role string,
		updatedTs int64,
	) error
	DeleteCalendarMember(ctx context.Context, accountID string, calendarUID uuid.UUID) error
	IsMemberOfCalendar(ctx context.Context, accountID string, calendarUID uuid.UUID) (bool, error)
	GetMemberRole(ctx context.Context, accountID string, calendarUID uuid.UUID) (string, error)
	IsCalendarOwner(ctx context.Context, accountID string, calendarUID uuid.UUID) (bool, error)
}

// AccountRepository defines the interface for account operations
type AccountRepository interface {
	DeleteAccount(ctx context.Context, accountID string) error
}

// AttendeeRepository defines the interface for attendee operations
type AttendeeRepository = *attendee.Queries

// Handler holds all repository dependencies
type Handler struct {
	AccountRepo        AccountRepository
	CalendarRepo       CalendarRepository
	CalendarMemberRepo CalendarMemberRepository
	EventRepo          EventRepository
	ReminderRepo       ReminderRepository
	AttendeeRepo       AttendeeRepository
	WebhookRepo        WebhookRepository
	WebhookDispatcher  *workers.WebhookDispatcher
	Config             *config.Config
}

// NewRestHandler creates a new Handler with the given repositories
func NewRestHandler(
	accountRepo AccountRepository,
	calendarRepo CalendarRepository,
	calendarMemberRepo CalendarMemberRepository,
	eventRepo EventRepository,
	reminderRepo ReminderRepository,
	attendeeRepo AttendeeRepository,
	webhookRepo WebhookRepository,
	webhookDispatcher *workers.WebhookDispatcher,
	cfg *config.Config,
) *Handler {
	return &Handler{
		AccountRepo:        accountRepo,
		CalendarRepo:       calendarRepo,
		CalendarMemberRepo: calendarMemberRepo,
		EventRepo:          eventRepo,
		ReminderRepo:       reminderRepo,
		AttendeeRepo:       attendeeRepo,
		WebhookRepo:        webhookRepo,
		WebhookDispatcher:  webhookDispatcher,
		Config:             cfg,
	}
}

// GenerationWindow returns the configured generation window for recurring events
func (h *Handler) GenerationWindow() event.GenerationWindow {
	return event.NewGenerationWindow(h.Config.GenerationWindow, h.Config.GenerationBuffer)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Calendar routes
	mux.HandleFunc("GET /api/v1/calendars", h.GetUserCalendars)
	mux.HandleFunc("POST /api/v1/calendars", h.CreateCalendar)
	mux.HandleFunc("GET /api/v1/calendars/{calendar_uid}", h.GetCalendar)
	mux.HandleFunc(
		"POST /api/v1/calendars/events",
		h.GetCalendarEvents,
	) // Query events across multiple calendars
	mux.HandleFunc("PUT /api/v1/calendars/{calendar_uid}", h.UpdateCalendar)
	mux.HandleFunc("DELETE /api/v1/calendars/{calendar_uid}", h.DeleteCalendar)

	// ICS Import
	mux.HandleFunc("POST /api/v1/calendars/import/ics", h.ImportICS)
	mux.HandleFunc("POST /api/v1/calendars/import/ics-link", h.ImportICSLink)
	mux.HandleFunc("POST /api/v1/calendars/{calendar_uid}/resync", h.ResyncCalendar)

	// Calendar member routes
	mux.HandleFunc("POST /api/v1/calendars/{calendar_uid}/members", h.InviteCalendarMembers)
	mux.HandleFunc("GET /api/v1/calendars/{calendar_uid}/members", h.GetCalendarMembers)
	mux.HandleFunc(
		"PUT /api/v1/calendars/{calendar_uid}/members/{account_id}",
		h.UpdateCalendarMember,
	)
	mux.HandleFunc(
		"DELETE /api/v1/calendars/{calendar_uid}/members/{account_id}",
		h.RemoveCalendarMember,
	)

	// Event routes
	mux.HandleFunc("POST /api/v1/events", h.CreateEvent)
	mux.HandleFunc("GET /api/v1/events/{event_uid}", h.GetEvent)
	mux.HandleFunc("PUT /api/v1/events/{event_uid}", h.UpdateEvent)
	mux.HandleFunc("DELETE /api/v1/events/{event_uid}", h.DeleteEvent)
	mux.HandleFunc("POST /api/v1/events/{event_uid}/toggle-cancelled", h.ToggleCancelledStatusEvent)

	// Reminder routes (nested under events)
	mux.HandleFunc("POST /api/v1/events/{event_uid}/reminders", h.CreateReminder)
	mux.HandleFunc("GET /api/v1/events/{event_uid}/reminders", h.GetEventReminders)
	mux.HandleFunc("PUT /api/v1/events/{event_uid}/reminders/{reminder_uid}", h.UpdateReminder)
	mux.HandleFunc("DELETE /api/v1/events/{event_uid}/reminders/{reminder_uid}", h.DeleteReminder)

	// Attendee routes (nested under events)
	mux.HandleFunc("POST /api/v1/events/{event_uid}/attendees", h.CreateAttendee)
	mux.HandleFunc("GET /api/v1/events/{event_uid}/attendees", h.GetEventAttendees)
	mux.HandleFunc("GET /api/v1/events/{event_uid}/attendees/{account_id}", h.GetAttendee)
	mux.HandleFunc("PATCH /api/v1/events/{event_uid}/attendees/{account_id}", h.UpdateAttendee)
	mux.HandleFunc("DELETE /api/v1/events/{event_uid}/attendees/{account_id}", h.DeleteAttendee)
	mux.HandleFunc(
		"PUT /api/v1/events/{event_uid}/attendees/{account_id}/rsvp",
		h.UpdateAttendeeRSVP,
	)
	mux.HandleFunc("POST /api/v1/events/{event_uid}/transfer-ownership", h.TransferEventOwnership)

	// Attendee event queries
	mux.HandleFunc("GET /api/v1/attendees/events", h.GetAttendeeEvents)

	// Webhook routes
	mux.HandleFunc("POST /api/v1/webhooks", h.CreateWebhook)
	mux.HandleFunc("GET /api/v1/webhooks/{webhook_uid}", h.GetWebhook)
	mux.HandleFunc("GET /api/v1/webhooks", h.GetWebhookEndpoints)
	mux.HandleFunc("PUT /api/v1/webhooks/{webhook_uid}", h.UpdateWebhook)
	mux.HandleFunc("DELETE /api/v1/webhooks/{webhook_uid}", h.DeleteWebhook)
	mux.HandleFunc("GET /api/v1/webhooks/deliveries/{webhook_uid}", h.GetWebhookDeliveries)

	// Account routes
	mux.HandleFunc("DELETE /api/v1/accounts", h.DeleteAccount)

	// Health check
	mux.HandleFunc("GET /health", h.HealthCheck)
}
