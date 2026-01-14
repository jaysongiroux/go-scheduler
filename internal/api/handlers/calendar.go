package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/api/web"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/calendar_member"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
	"github.com/jaysongiroux/go-scheduler/internal/workers"
)

func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	var cal calendar.Calendar
	if err := json.NewDecoder(r.Body).Decode(&cal); err != nil {
		logger.Warn("Failed to decode calendar: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if cal.CalendarUID == uuid.Nil {
		cal.CalendarUID = uuid.New()
	}
	if cal.Settings == nil {
		cal.Settings = json.RawMessage(`{}`)
	}
	if cal.Metadata == nil {
		cal.Metadata = json.RawMessage(`{}`)
	}

	now := time.Now().Unix()
	cal.CreatedTs = now
	cal.UpdatedTs = now

	if err := h.CalendarRepo.CreateCalendar(r.Context(), &cal); err != nil {
		logger.Error("Failed to create calendar: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to create calendar",
			err.Error(),
		)
		return
	}

	web.RespondJSON(w, http.StatusCreated, cal)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.CalendarCreated,
		cal,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for calendar creation: %v", err)
	}
}

func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	cal, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Warn("Calendar not found: %v", err)
		web.RespondError(w, http.StatusNotFound, "Calendar not found", err.Error())
		return
	}

	// Get members for this calendar
	members, err := h.CalendarMemberRepo.GetCalendarMembers(r.Context(), calendarUID)
	if err != nil {
		logger.Error("Failed to get calendar members: %v", err)
		// Don't fail the request, just return empty members
		members = []*calendar_member.CalendarMember{}
	}

	// Create response with all calendar fields plus members
	response := struct {
		*calendar.Calendar
		Members []*calendar_member.CalendarMember `json:"members"`
	}{
		Calendar: cal,
		Members:  members,
	}

	web.RespondJSON(w, http.StatusOK, response)
}

func (h *Handler) GetUserCalendars(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		web.RespondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	limitInt, offsetInt, err := ExtractLimitOffset(r, h.Config)
	if err != nil {
		web.RespondError(w, http.StatusBadRequest, "Invalid limit or offset", err.Error())
		return
	}

	// Get calendars owned by the user
	ownedCalendars, err := h.CalendarRepo.GetUserCalendars(
		r.Context(),
		accountID,
		limitInt,
		offsetInt,
	)
	if err != nil {
		logger.Error("Failed to get owned calendars: %v", err)
		web.RespondError(w, http.StatusInternalServerError, "Failed to get calendars", err.Error())
		return
	}

	// Get calendars where user is a member
	memberCalendarUIDs, err := h.CalendarMemberRepo.GetMemberCalendars(
		r.Context(),
		accountID,
		limitInt,
		offsetInt,
	)
	if err != nil {
		logger.Error("Failed to get member calendars: %v", err)
		// Don't fail the request, just continue with owned calendars
		memberCalendarUIDs = []uuid.UUID{}
	}

	// Fetch full calendar details for member calendars
	memberCalendars := make([]*calendar.Calendar, 0, len(memberCalendarUIDs))
	for _, calUID := range memberCalendarUIDs {
		cal, err := h.CalendarRepo.GetCalendar(r.Context(), calUID)
		if err != nil {
			logger.Warn("Failed to get member calendar %s: %v", calUID, err)
			continue
		}
		memberCalendars = append(memberCalendars, cal)
	}

	// Create response with ownership indicator and members
	type CalendarWithMembers struct {
		*calendar.Calendar
		IsOwner bool                              `json:"is_owner"`
		Members []*calendar_member.CalendarMember `json:"members"`
	}

	allCalendars := make([]CalendarWithMembers, 0, len(ownedCalendars)+len(memberCalendars))

	// Add owned calendars
	for _, cal := range ownedCalendars {
		members, err := h.CalendarMemberRepo.GetCalendarMembers(r.Context(), cal.CalendarUID)
		if err != nil {
			logger.Warn("Failed to get members for calendar %s: %v", cal.CalendarUID, err)
			members = []*calendar_member.CalendarMember{}
		}

		allCalendars = append(allCalendars, CalendarWithMembers{
			Calendar: cal,
			IsOwner:  true,
			Members:  members,
		})
	}

	// Add member calendars
	for _, cal := range memberCalendars {
		members, err := h.CalendarMemberRepo.GetCalendarMembers(r.Context(), cal.CalendarUID)
		if err != nil {
			logger.Warn("Failed to get members for calendar %s: %v", cal.CalendarUID, err)
			members = []*calendar_member.CalendarMember{}
		}

		allCalendars = append(allCalendars, CalendarWithMembers{
			Calendar: cal,
			IsOwner:  false,
			Members:  members,
		})
	}

	web.ResponsePagedResults(w, allCalendars, len(allCalendars), limitInt, offsetInt)
}

func (h *Handler) UpdateCalendar(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if calendarUID == uuid.Nil {
		logger.Warn("Calendar UID is required")
		web.RespondError(w, http.StatusBadRequest, "Calendar UID is required", err.Error())
		return
	}

	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	var cal calendar.Calendar
	if err := json.NewDecoder(r.Body).Decode(&cal); err != nil {
		logger.Warn("Failed to decode calendar: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	cal.CalendarUID = calendarUID
	cal.UpdatedTs = time.Now().Unix()

	if len(cal.Settings) == 0 || string(cal.Settings) == "null" {
		cal.Settings = json.RawMessage(`{}`)
	}
	if len(cal.Metadata) == 0 || string(cal.Metadata) == "null" {
		cal.Metadata = json.RawMessage(`{}`)
	}

	if err := h.CalendarRepo.UpdateCalendar(r.Context(), &cal); err != nil {
		logger.Error("Failed to update calendar: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to update calendar",
			err.Error(),
		)
		return
	}

	updatedCal, err := h.CalendarRepo.GetCalendar(r.Context(), calendarUID)
	if err != nil {
		logger.Error("Failed to get updated calendar: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to get updated calendar",
			err.Error(),
		)
		return
	}

	web.RespondJSON(w, http.StatusOK, updatedCal)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.CalendarUpdated,
		updatedCal,
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for calendar update: %v", err)
	}
}

func (h *Handler) DeleteCalendar(w http.ResponseWriter, r *http.Request) {
	calendarUID, err := uuid.Parse(r.PathValue("calendar_uid"))
	if err != nil {
		logger.Warn("Failed to parse calendar UID: %v", err)
		web.RespondError(w, http.StatusBadRequest, "Invalid calendar UID", err.Error())
		return
	}

	if err := h.CalendarRepo.DeleteCalendar(r.Context(), calendarUID); err != nil {
		logger.Error("Failed to delete calendar: %v", err)
		web.RespondError(
			w,
			http.StatusInternalServerError,
			"Failed to delete calendar",
			err.Error(),
		)
		return
	}

	web.ResponseSuccess(w)

	// Queue webhook delivery
	if err := h.WebhookDispatcher.QueueDelivery(
		r.Context(),
		workers.CalendarDeleted,
		map[string]interface{}{
			"calendar_uid": calendarUID.String(),
		},
		nil,
	); err != nil {
		logger.Error("Failed to queue webhook for calendar deletion: %v", err)
	}
}
