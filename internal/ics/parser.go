package ics

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// ICSCalendar represents a parsed ICS calendar
type ICSCalendar struct {
	Events   []ICSEvent
	Timezone string
}

// ICSEvent represents a parsed ICS event
type ICSEvent struct {
	UID            string
	Summary        string
	Description    string
	Location       string
	StartTime      time.Time
	EndTime        time.Time
	IsAllDay       bool
	RecurrenceRule string
	ExDates        []time.Time
	Attendees      []ICSAttendee
	Reminders      []ICSReminder
	Timezone       string
	Status         string
	URL            string
	Created        time.Time
	LastModified   time.Time
}

// ICSAttendee represents an attendee from an ICS event
type ICSAttendee struct {
	Email  string
	Name   string
	Role   string // REQ-PARTICIPANT, OPT-PARTICIPANT, etc.
	Status string // NEEDS-ACTION, ACCEPTED, DECLINED, TENTATIVE
}

// ICSReminder represents a reminder/alarm from an ICS event
type ICSReminder struct {
	Trigger     string // e.g., "-PT15M" (15 minutes before)
	Action      string // DISPLAY, EMAIL, etc.
	Description string
}

// Parser handles ICS file parsing
type Parser struct{}

// NewParser creates a new ICS parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseICS parses an ICS file from a reader and returns structured data
func (p *Parser) ParseICS(reader io.Reader) (*ICSCalendar, error) {
	decoder := ical.NewDecoder(reader)
	cal, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode ICS file: %w", err)
	}

	return p.convertCalendar(cal)
}

func (p *Parser) convertCalendar(cal *ical.Calendar) (*ICSCalendar, error) {
	result := &ICSCalendar{
		Events: make([]ICSEvent, 0),
	}

	// Extract calendar timezone if available
	for _, child := range cal.Children {
		if child.Name == ical.CompTimezone {
			if tzid := child.Props.Get(ical.PropTimezoneID); tzid != nil {
				result.Timezone = tzid.Value
			}
		}
	}

	// Extract events
	for _, child := range cal.Children {
		if child.Name == ical.CompEvent {
			event, err := p.parseEvent(child, result.Timezone)
			if err != nil {
				// Log error but continue with other events
				continue
			}
			result.Events = append(result.Events, event)
		}
	}

	return result, nil
}

func (p *Parser) parseEvent(comp *ical.Component, defaultTZ string) (ICSEvent, error) {
	event := ICSEvent{
		Timezone: defaultTZ,
	}

	// Parse UID
	if prop := comp.Props.Get(ical.PropUID); prop != nil {
		event.UID = prop.Value
	}

	// Parse summary (title)
	if prop := comp.Props.Get(ical.PropSummary); prop != nil {
		event.Summary = prop.Value
	}

	// Parse description
	if prop := comp.Props.Get(ical.PropDescription); prop != nil {
		event.Description = prop.Value
	}

	// Parse location
	if prop := comp.Props.Get(ical.PropLocation); prop != nil {
		event.Location = prop.Value
	}

	// Parse start time
	if prop := comp.Props.Get(ical.PropDateTimeStart); prop != nil {
		startTime, isAllDay, tz := p.parseDateTime(prop, defaultTZ)
		event.StartTime = startTime
		event.IsAllDay = isAllDay
		if tz != "" {
			event.Timezone = tz
		}
	}

	// Parse end time
	if prop := comp.Props.Get(ical.PropDateTimeEnd); prop != nil {
		endTime, _, _ := p.parseDateTime(prop, defaultTZ)
		event.EndTime = endTime
	} else if prop := comp.Props.Get(ical.PropDuration); prop != nil {
		// Calculate end time from duration
		duration, err := ParseISO8601Duration(prop.Value)
		if err == nil {
			event.EndTime = event.StartTime.Add(duration)
		}
	} else if event.IsAllDay {
		// All-day events default to 1 day duration
		event.EndTime = event.StartTime.Add(24 * time.Hour)
	} else {
		// Default to same as start time if no end specified
		event.EndTime = event.StartTime
	}

	// Parse recurrence rule
	if prop := comp.Props.Get(ical.PropRecurrenceRule); prop != nil {
		event.RecurrenceRule = prop.Value
	}

	// Parse EXDATE (exclusion dates)
	event.ExDates = p.parseExDates(comp, defaultTZ)

	// Parse status
	if prop := comp.Props.Get(ical.PropStatus); prop != nil {
		event.Status = prop.Value
	}

	// Parse URL
	if prop := comp.Props.Get(ical.PropURL); prop != nil {
		event.URL = prop.Value
	}

	// Parse attendees
	event.Attendees = p.parseAttendees(comp)

	// Parse alarms (reminders)
	event.Reminders = p.parseAlarms(comp)

	// Parse created timestamp
	if prop := comp.Props.Get(ical.PropCreated); prop != nil {
		if created, _, _ := p.parseDateTime(prop, defaultTZ); !created.IsZero() {
			event.Created = created
		}
	}

	// Parse last modified timestamp
	if prop := comp.Props.Get(ical.PropLastModified); prop != nil {
		if modified, _, _ := p.parseDateTime(prop, defaultTZ); !modified.IsZero() {
			event.LastModified = modified
		}
	}

	return event, nil
}

func (p *Parser) parseDateTime(prop *ical.Prop, defaultTZ string) (time.Time, bool, string) {
	if prop == nil {
		return time.Time{}, false, ""
	}

	// Check if it's an all-day event (VALUE=DATE)
	valueParam := prop.Params.Get(ical.ParamValue)
	isAllDay := valueParam == "DATE"

	// Get timezone from TZID parameter
	tzid := prop.Params.Get(ical.ParamTimezoneID)
	if tzid == "" {
		tzid = defaultTZ
	}

	var loc *time.Location
	if tzid != "" {
		var err error
		loc, err = time.LoadLocation(tzid)
		if err != nil {
			loc = time.UTC
		}
	} else {
		loc = time.UTC
	}

	var t time.Time
	var err error

	if isAllDay {
		// Parse date-only format: 20240115
		t, err = time.ParseInLocation("20060102", prop.Value, loc)
	} else if strings.HasSuffix(prop.Value, "Z") {
		// UTC time
		t, err = time.Parse("20060102T150405Z", prop.Value)
	} else {
		// Local time with timezone
		t, err = time.ParseInLocation("20060102T150405", prop.Value, loc)
	}

	if err != nil {
		return time.Time{}, false, ""
	}

	return t, isAllDay, tzid
}

func (p *Parser) parseExDates(comp *ical.Component, defaultTZ string) []time.Time {
	var exdates []time.Time

	for _, prop := range comp.Props.Values(ical.PropExceptionDates) {
		// EXDATE can have multiple comma-separated values
		values := strings.Split(prop.Value, ",")
		for _, v := range values {
			tempProp := &ical.Prop{
				Name:   ical.PropExceptionDates,
				Value:  strings.TrimSpace(v),
				Params: prop.Params,
			}
			if t, _, _ := p.parseDateTime(tempProp, defaultTZ); !t.IsZero() {
				exdates = append(exdates, t)
			}
		}
	}

	return exdates
}

func (p *Parser) parseAttendees(comp *ical.Component) []ICSAttendee {
	var attendees []ICSAttendee

	for _, prop := range comp.Props.Values(ical.PropAttendee) {
		attendee := ICSAttendee{
			Email:  extractEmail(prop.Value),
			Status: "NEEDS-ACTION",
		}

		if cn := prop.Params.Get(ical.ParamCommonName); cn != "" {
			attendee.Name = cn
		}

		if role := prop.Params.Get(ical.ParamRole); role != "" {
			attendee.Role = role
		}

		if status := prop.Params.Get(ical.ParamParticipationStatus); status != "" {
			attendee.Status = status
		}

		attendees = append(attendees, attendee)
	}

	return attendees
}

func (p *Parser) parseAlarms(comp *ical.Component) []ICSReminder {
	var reminders []ICSReminder

	for _, child := range comp.Children {
		if child.Name == ical.CompAlarm {
			reminder := ICSReminder{
				Action: "DISPLAY",
			}

			if prop := child.Props.Get(ical.PropTrigger); prop != nil {
				reminder.Trigger = prop.Value
			}

			if prop := child.Props.Get(ical.PropAction); prop != nil {
				reminder.Action = prop.Value
			}

			if prop := child.Props.Get(ical.PropDescription); prop != nil {
				reminder.Description = prop.Value
			}

			if reminder.Trigger != "" {
				reminders = append(reminders, reminder)
			}
		}
	}

	return reminders
}

func extractEmail(value string) string {
	// Remove "mailto:" prefix if present
	if strings.HasPrefix(strings.ToLower(value), "mailto:") {
		return value[7:]
	}
	return value
}
