package main

import (
	"strings"
	"testing"
)

// TestPruneConsentAnswer guards the prune prompt: only a line that is exactly
// "y" or "yes" (case-insensitive, surrounding blanks trimmed) consents. The
// old Fscanln read stopped at the first word, so "y extra" deleted sessions.
func TestPruneConsentAnswer(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]bool{
		"y\n": true, "Y\n": true, "yes\n": true, " Yes \n": true,
		"y extra\n": false, "yes please\n": false, "n\n": false, "\n": false, "": false, "yy\n": false,
	} {
		if got := readConsent(strings.NewReader(input)); got != want {
			t.Errorf("readConsent(%q) = %v, want %v", input, got, want)
		}
	}
}
