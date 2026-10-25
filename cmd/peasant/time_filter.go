package main

import (
	"fmt"
	"strconv"
	"time"
)

// timeFilterHelp is the grammar every --since/--until/--before/--after flag
// accepts, for flag help text.
const timeFilterHelp = "Nh, Nd, Nw, Nm (months), Ny, or YYYY-MM-DD"

// parseTimeFilter is the one parser behind every --since/--until/--before/
// --after flag: a date (YYYY-MM-DD or RFC3339) or a relative "N ago" with
// unit h, d, w, m (months) or y. now anchors relative values.
func parseTimeFilter(s string, now time.Time) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return parseRelativeAgo(s, now)
}

// parseRelativeAgo parses "N<unit>" and returns now minus that span.
func parseRelativeAgo(s string, now time.Time) (time.Time, error) {
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid duration: %q (expected %s)", s, timeFilterHelp)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid duration: %q (number must be positive integer; expected %s)", s, timeFilterHelp)
	}
	switch s[len(s)-1] {
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	case 'm':
		return now.AddDate(0, -n, 0), nil
	case 'y':
		return now.AddDate(-n, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid duration unit %q in %q (expected %s)", s[len(s)-1:], s, timeFilterHelp)
	}
}
