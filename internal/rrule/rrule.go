package rrule

import (
	"errors"

	"github.com/teambition/rrule-go"
)

// ErrInvalidRecurrenceRule is returned when a recurrence rule is invalid
var ErrInvalidRecurrenceRule = errors.New("invalid recurrence rule")

// Recurrence defines how an event repeats.
type Recurrence struct {
	// Rule is the iCal RRULE string (e.g., "FREQ=WEEKLY;BYDAY=MO,WE,FR")
	Rule string `json:"rule"`
}

// validates
func ValidateRRule(r *Recurrence) error {
	if r == nil || r.Rule == "" {
		return nil
	}

	opt, err := rrule.StrToROption(r.Rule)
	if err != nil {
		return ErrInvalidRecurrenceRule
	}

	_, err = rrule.NewRRule(*opt)
	if err != nil {
		return ErrInvalidRecurrenceRule
	}

	return nil
}

// IsEmpty returns true if the recurrence has no valid rule
func IsRRuleEmpty(r *Recurrence) bool {
	return r == nil || r.Rule == ""
}
