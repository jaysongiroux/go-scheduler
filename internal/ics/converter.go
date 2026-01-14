package ics

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jaysongiroux/go-scheduler/internal/db/services/event"
	"github.com/jaysongiroux/go-scheduler/internal/rrule"
)

// Converter converts ICS events to internal event format
type Converter struct{}

// NewConverter creates a new converter
func NewConverter() *Converter {
	return &Converter{}
}

// ConvertedEvent holds an event and its associated reminders/attendees
type ConvertedEvent struct {
	Event     *event.Event
	Reminders []ConvertedReminder
	Attendees []ConvertedAttendee
}

// ConvertedReminder holds reminder data for import
type ConvertedReminder struct {
	OffsetSeconds int64
	Metadata      json.RawMessage
}

// ConvertedAttendee holds attendee data for import
type ConvertedAttendee struct {
	AccountID  string
	Role       string
	RSVPStatus string
	Metadata   json.RawMessage
}

// ConvertToEvent converts an ICS event to the internal event format
func (c *Converter) ConvertToEvent(
	icsEvent ICSEvent,
	calendarUID uuid.UUID,
	accountID string,
) (*ConvertedEvent, error) {
	now := time.Now().Unix()

	evt := &event.Event{
		EventUID:    uuid.New(),
		CalendarUID: calendarUID,
		AccountID:   accountID,
		StartTs:     icsEvent.StartTime.Unix(),
		EndTs:       icsEvent.EndTime.Unix(),
		Duration:    icsEvent.EndTime.Unix() - icsEvent.StartTime.Unix(),
		CreatedTs:   now,
		UpdatedTs:   now,
	}

	// Set timezone
	if icsEvent.Timezone != "" {
		evt.Timezone = &icsEvent.Timezone
	}

	// Set local_start for timezone-aware scheduling
	if icsEvent.Timezone != "" {
		loc, err := time.LoadLocation(icsEvent.Timezone)
		if err == nil {
			localStart := icsEvent.StartTime.In(loc).Format("2006-01-02T15:04:05")
			evt.LocalStart = &localStart
		}
	}

	// Convert recurrence rule
	if icsEvent.RecurrenceRule != "" {
		evt.Recurrence = &rrule.Recurrence{
			Rule: "RRULE:" + icsEvent.RecurrenceRule,
		}
		status := event.RecurrenceStatusActive
		evt.RecurrenceStatus = &status
	}

	// Convert EXDATES
	if len(icsEvent.ExDates) > 0 {
		evt.ExDatesTs = make([]int64, len(icsEvent.ExDates))
		for i, exdate := range icsEvent.ExDates {
			evt.ExDatesTs[i] = exdate.Unix()
		}
	}

	// Build metadata
	metadata := map[string]interface{}{
		"ics_uid": icsEvent.UID,
	}

	if icsEvent.Summary != "" {
		metadata["title"] = icsEvent.Summary
	}
	if icsEvent.Description != "" {
		metadata["description"] = icsEvent.Description
	}
	if icsEvent.Location != "" {
		metadata["location"] = icsEvent.Location
	}
	if icsEvent.IsAllDay {
		metadata["is_all_day"] = true
	}
	if icsEvent.Status != "" {
		metadata["status"] = icsEvent.Status
	}
	if icsEvent.URL != "" {
		metadata["url"] = icsEvent.URL
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	evt.Metadata = metadataJSON

	result := &ConvertedEvent{
		Event:     evt,
		Reminders: c.convertReminders(icsEvent.Reminders),
		Attendees: c.convertAttendees(icsEvent.Attendees),
	}

	return result, nil
}

func (c *Converter) convertReminders(icsReminders []ICSReminder) []ConvertedReminder {
	reminders := make([]ConvertedReminder, 0, len(icsReminders))

	for _, icsReminder := range icsReminders {
		// Parse trigger duration
		duration, err := ParseISO8601Duration(icsReminder.Trigger)
		if err != nil {
			continue
		}

		offsetSeconds := DurationToOffsetSeconds(duration)

		// Reminders should have negative offset (before event)
		// Some ICS files may have positive triggers, skip those
		if offsetSeconds > 0 {
			continue
		}

		metadata := map[string]interface{}{}
		if icsReminder.Action != "" {
			metadata["action"] = icsReminder.Action
		}
		if icsReminder.Description != "" {
			metadata["description"] = icsReminder.Description
		}

		metadataJSON, _ := json.Marshal(metadata)

		reminders = append(reminders, ConvertedReminder{
			OffsetSeconds: offsetSeconds,
			Metadata:      metadataJSON,
		})
	}

	return reminders
}

func (c *Converter) convertAttendees(icsAttendees []ICSAttendee) []ConvertedAttendee {
	attendees := make([]ConvertedAttendee, 0, len(icsAttendees))

	for _, icsAttendee := range icsAttendees {
		// Use email as account_id
		if icsAttendee.Email == "" {
			continue
		}

		// Map ICS role to internal role
		role := "attendee"
		if icsAttendee.Role == "CHAIR" {
			role = "organizer"
		}

		// Map ICS status to internal RSVP status
		rsvpStatus := mapRSVPStatus(icsAttendee.Status)

		metadata := map[string]interface{}{}
		if icsAttendee.Name != "" {
			metadata["name"] = icsAttendee.Name
		}
		if icsAttendee.Role != "" {
			metadata["ics_role"] = icsAttendee.Role
		}

		metadataJSON, _ := json.Marshal(metadata)

		attendees = append(attendees, ConvertedAttendee{
			AccountID:  icsAttendee.Email,
			Role:       role,
			RSVPStatus: rsvpStatus,
			Metadata:   metadataJSON,
		})
	}

	return attendees
}

func mapRSVPStatus(icsStatus string) string {
	switch icsStatus {
	case "ACCEPTED":
		return "accepted"
	case "DECLINED":
		return "declined"
	case "TENTATIVE":
		return "tentative"
	default:
		return "pending"
	}
}
