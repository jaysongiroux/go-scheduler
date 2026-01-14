package ics

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseISO8601Duration parses an ISO 8601 duration string and returns a time.Duration.
// Supports formats like: PT15M, PT1H, P1D, P1DT2H30M, -PT15M (negative)
func ParseISO8601Duration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}

	if !strings.HasPrefix(s, "P") {
		return 0, fmt.Errorf("invalid duration format: must start with P")
	}
	s = s[1:] // Remove P prefix

	var totalDuration time.Duration

	// Split by T to separate date and time parts
	parts := strings.SplitN(s, "T", 2)
	datePart := parts[0]
	timePart := ""
	if len(parts) > 1 {
		timePart = parts[1]
	}

	// Parse date part (days, weeks, months, years)
	if datePart != "" {
		d, err := parseDatePart(datePart)
		if err != nil {
			return 0, err
		}
		totalDuration += d
	}

	// Parse time part (hours, minutes, seconds)
	if timePart != "" {
		d, err := parseTimePart(timePart)
		if err != nil {
			return 0, err
		}
		totalDuration += d
	}

	if negative {
		totalDuration = -totalDuration
	}

	return totalDuration, nil
}

func parseDatePart(s string) (time.Duration, error) {
	var total time.Duration
	re := regexp.MustCompile(`(\d+)([DWMY])`)
	matches := re.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		value, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", match[1])
		}

		switch match[2] {
		case "D":
			total += time.Duration(value) * 24 * time.Hour
		case "W":
			total += time.Duration(value) * 7 * 24 * time.Hour
		case "M":
			// Approximate month as 30 days
			total += time.Duration(value) * 30 * 24 * time.Hour
		case "Y":
			// Approximate year as 365 days
			total += time.Duration(value) * 365 * 24 * time.Hour
		}
	}

	return total, nil
}

func parseTimePart(s string) (time.Duration, error) {
	var total time.Duration
	re := regexp.MustCompile(`(\d+)([HMS])`)
	matches := re.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		value, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", match[1])
		}

		switch match[2] {
		case "H":
			total += time.Duration(value) * time.Hour
		case "M":
			total += time.Duration(value) * time.Minute
		case "S":
			total += time.Duration(value) * time.Second
		}
	}

	return total, nil
}

// DurationToOffsetSeconds converts a time.Duration to seconds offset
// Used for reminder triggers (negative values = before event)
func DurationToOffsetSeconds(d time.Duration) int64 {
	return int64(d.Seconds())
}
