package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/account"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/webhook"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

// CalendarRepository defines the interface for calendar operations
type CalendarRepository interface {
	CreateCalendar(ctx context.Context, cal *calendar.Calendar) error
	GetCalendar(ctx context.Context, calendarUID uuid.UUID) (*calendar.Calendar, error)
	GetUserCalendars(ctx context.Context, userID string, limit int, offset int) ([]*calendar.Calendar, error)
	UpdateCalendar(ctx context.Context, cal *calendar.Calendar) error
	DeleteCalendar(ctx context.Context, calendarUID uuid.UUID) error
}

// WebhookRepository defines the interface for webhook operations
type WebhookRepository interface {
	CreateWebhook(ctx context.Context, w *webhook.Webhook) error
	GetWebhook(ctx context.Context, webhookUID uuid.UUID) (*webhook.Webhook, error)
	GetWebhookEndpoints(ctx context.Context, offset, limit int) ([]*webhook.Webhook, error)
	UpdateWebhook(ctx context.Context, w *webhook.Webhook) error
	DeleteWebhook(ctx context.Context, webhookUID uuid.UUID) error
	GetWebhookDeliveries(ctx context.Context, webhookUID uuid.UUID, limit int, offset int) ([]*webhook.WebhookDelivery, error)
}

// AccountRepository defines the interface for account operations
type AccountRepository interface {
	CheckAccountExists(ctx context.Context, accountID string) (bool, error)
	CreateAccount(ctx context.Context, acc *account.Account) error
	GetAccountByID(ctx context.Context, accountID string) (*account.Account, error)
	DeleteAccountByID(ctx context.Context, accountID string) error
	UpdateAccountByID(ctx context.Context, accountID string, acc *account.Account) error
	GetAccounts(
		ctx context.Context,
		limit int,
		offset int,
		filters *account.AccountFilters,
	) ([]*account.Account, error)
}

// EventRepository defines the interface for event operations
type EventRepository interface {
	CreateEvent(ctx context.Context, evt *event.Event) (event.Event, error)
	GetEvent(ctx context.Context, eventUID uuid.UUID, includeArchivedReminders *bool) (*event.Event, error)
	GetCalendarEvents(
		ctx context.Context,
		calendarUID uuid.UUID,
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
	GetFutureInstances(ctx context.Context, masterUID uuid.UUID, fromTs int64) ([]*event.Event, error)
	AddExDate(ctx context.Context, masterUID uuid.UUID, exdateTs int64) error
	CancelInstance(ctx context.Context, instanceUID uuid.UUID) error
	ToggleCancelledStatusEvent(ctx context.Context, eventUID uuid.UUID) error
	CountInstancesByMaster(ctx context.Context, masterUID uuid.UUID) (int, error)
}

// ReminderRepository defines the interface for reminder operations
type ReminderRepository interface {
	CreateSingleReminder(ctx context.Context, reminder *event.Reminder) (*event.Reminder, error)
	CreateSeriesReminder(ctx context.Context, reminder *event.Reminder) (uuid.UUID, int, error)
	UpdateSingleReminder(ctx context.Context, reminder *event.Reminder) error
	UpdateSeriesReminder(ctx context.Context, reminder *event.Reminder) (int, error)
	DeleteSingleReminder(ctx context.Context, reminderUID uuid.UUID) error
	DeleteSeriesReminder(ctx context.Context, reminderUID uuid.UUID) (int, error)
	GetEventReminders(ctx context.Context, eventUID uuid.UUID, accountID *string) ([]*event.Reminder, error)
	CopyRemindersToNewEvent(ctx context.Context, masterEventUID, newEventUID uuid.UUID) (int, error)
	GetReminderByUID(ctx context.Context, reminderUID uuid.UUID) (*event.Reminder, error)
	MarkReminderDelivered(ctx context.Context, reminderUID uuid.UUID) error
	GetDueReminders(ctx context.Context, beforeTs int64, limit int) ([]*event.Reminder, error)
	GetDueRemindersWithEvents(ctx context.Context, beforeTs int64, limit int) ([]*event.ReminderWithEvent, error)
}

// Handler holds all repository dependencies
type Handler struct {
	CalendarRepo      CalendarRepository
	EventRepo         EventRepository
	ReminderRepo      ReminderRepository
	WebhookRepo       WebhookRepository
	AccountRepo       AccountRepository
	WebhookDispatcher *workers.WebhookDispatcher
	Config            *config.Config
}

// NewRestHandler creates a new Handler with the given repositories
func NewRestHandler(
	calendarRepo CalendarRepository,
	eventRepo EventRepository,
	reminderRepo ReminderRepository,
	webhookRepo WebhookRepository,
	accountRepo AccountRepository,
	webhookDispatcher *workers.WebhookDispatcher,
	cfg *config.Config,
) *Handler {
	return &Handler{
		CalendarRepo:      calendarRepo,
		EventRepo:         eventRepo,
		ReminderRepo:      reminderRepo,
		WebhookRepo:       webhookRepo,
		AccountRepo:       accountRepo,
		WebhookDispatcher: webhookDispatcher,
		Config:            cfg,
	}
}

// GenerationWindow returns the configured generation window for recurring events
func (h *Handler) GenerationWindow() event.GenerationWindow {
	return event.NewGenerationWindow(h.Config.GenerationWindow, h.Config.GenerationBuffer)
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// account routes
	mux.HandleFunc("POST /api/v1/accounts", h.CreateAccount)
	mux.HandleFunc("GET /api/v1/accounts/{account_id}", h.GetAccountByID)
	mux.HandleFunc("DELETE /api/v1/accounts/{account_id}", h.DeleteAccountByID)
	mux.HandleFunc("PUT /api/v1/accounts/{account_id}", h.UpdateAccountByID)
	mux.HandleFunc("GET /api/v1/accounts", h.GetAccounts)
	mux.HandleFunc("GET /api/v1/account/{account_id}/calendars", h.GetUserCalendars)

	// Calendar routes
	mux.HandleFunc("POST /api/v1/calendars", h.CreateCalendar)
	mux.HandleFunc("GET /api/v1/calendars/{calendar_uid}", h.GetCalendar)
	mux.HandleFunc("GET /api/v1/calendars/{calendar_uid}/events", h.GetCalendarEvents) // Query events by calendar_uid param
	mux.HandleFunc("PUT /api/v1/calendars/{calendar_uid}", h.UpdateCalendar)
	mux.HandleFunc("DELETE /api/v1/calendars/{calendar_uid}", h.DeleteCalendar)

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

	// Webhook routes
	mux.HandleFunc("POST /api/v1/webhooks", h.CreateWebhook)
	mux.HandleFunc("GET /api/v1/webhooks/{webhook_uid}", h.GetWebhook)
	mux.HandleFunc("GET /api/v1/webhooks", h.GetWebhookEndpoints)
	mux.HandleFunc("PUT /api/v1/webhooks/{webhook_uid}", h.UpdateWebhook)
	mux.HandleFunc("DELETE /api/v1/webhooks/{webhook_uid}", h.DeleteWebhook)
	mux.HandleFunc("GET /api/v1/webhooks/deliveries/{webhook_uid}", h.GetWebhookDeliveries)

	// Health check
	mux.HandleFunc("GET /health", h.HealthCheck)
}
