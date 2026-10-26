package main

import (
	"strings"
	"testing"
	"time"
)

// TestTimeFiltersShareOneGrammar proves harvest --since, sessions list
// --since and prune --before/--after read the same relative grammar
// (h/d/w/mo/y; bare m = months outside prune) and the same dates, through the functions those
// commands call.
func TestTimeFiltersShareOneGrammar(t *testing.T) {
	t.Parallel()
	daysAgo := func(ms int64) int { return int(time.Since(time.UnixMilli(ms)).Hours() / 24) }
	for _, tc := range []struct {
		in       string
		min, max int
	}{
		{"3w", 20, 22},
		{"7d", 6, 8},
		{"3mo", 88, 93},
		{"1y", 364, 367},
	} {
		h, err := parseSinceDuration(tc.in)
		if err != nil {
			t.Errorf("harvest --since %s: %v", tc.in, err)
		} else if d := daysAgo(h.UnixMilli()); d < tc.min || d > tc.max {
			t.Errorf("harvest --since %s: %d days ago, want %d-%d", tc.in, d, tc.min, tc.max)
		}
		s, err := parseDateOrDuration(tc.in, true)
		if err != nil {
			t.Errorf("sessions list --since %s: %v", tc.in, err)
		} else if d := daysAgo(s); d < tc.min || d > tc.max {
			t.Errorf("sessions list --since %s: %d days ago, want %d-%d", tc.in, d, tc.min, tc.max)
		}
		f, err := buildPruneFilter(nil, "", "", tc.in, tc.in, false)
		if err != nil {
			t.Errorf("prune --before/--after %s: %v", tc.in, err)
			continue
		}
		for name, v := range map[string]*int64{"before": f.Before, "after": f.After} {
			if v == nil {
				t.Errorf("prune --%s %s: unset", name, tc.in)
			} else if d := daysAgo(*v); d < tc.min || d > tc.max {
				t.Errorf("prune --%s %s: %d days ago, want %d-%d", name, tc.in, d, tc.min, tc.max)
			}
		}
	}
	h, err := parseSinceDuration("2026-01-02")
	if err != nil || h.Format("2006-01-02") != "2026-01-02" {
		t.Errorf("harvest --since 2026-01-02 = %v, %v", h, err)
	}
	f, err := buildPruneFilter(nil, "", "", "2026-01-02", "", false)
	if err != nil || time.UnixMilli(*f.Before).UTC().Format("2006-01-02") != "2026-01-02" {
		t.Errorf("prune --before 2026-01-02: %v", err)
	}
}

// TestPruneCmdAcceptsRelativeBefore runs the real prune command with the
// relative form harvest documents.
func TestPruneCmdAcceptsRelativeBefore(t *testing.T) {
	t.Parallel()
	if _, _, err := executePruneCmd(t, t.TempDir(), []string{"--before", "7d", "--dry-run"}); err != nil {
		t.Fatalf("prune --before 7d --dry-run: %v", err)
	}
}

// TestPruneRefusesTheAmbiguousMonthUnit proves the destructive prune command
// refuses a bare "m" (read by people as minutes, by the parser as months)
// with a hint, and takes the explicit "mo".
func TestPruneRefusesTheAmbiguousMonthUnit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneTestSessions(t, dir)
	for _, flag := range []string{"--after", "--before"} {
		out, _, err := executePruneCmd(t, dir, []string{flag, "30m", "--dry-run"})
		if err == nil || !strings.Contains(err.Error(), "30mo") || strings.Contains(out, "session(s) would be deleted") {
			t.Fatalf("prune %s 30m: err %v, want a refusal naming 30mo and no selection:\n%s", flag, err, out)
		}
	}
	// Outside prune a bare m keeps meaning months (a maintainer decision).
	if s, err := parseDateOrDuration("3m", true); err != nil || time.Since(time.UnixMilli(s)).Hours()/24 < 88 {
		t.Fatalf("sessions list --since 3m: %v, want about three months ago", err)
	}
	if _, _, err := executePruneCmd(t, dir, []string{"--after", "30mo", "--dry-run"}); err != nil {
		t.Fatalf("prune --after 30mo: %v", err)
	}
}
