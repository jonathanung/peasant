package main

import (
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
)

// TestSessionDetailShowsEachClaudeReplyOnce guards #683: a single-block
// Claude assistant message is stored as the message and its one text block,
// and the detail API served both, so the e2e fixture's 4 records came back
// as 6 turns, each reply repeated with a zero timestamp.
func TestSessionDetailShowsEachClaudeReplyOnce(t *testing.T) {
	t.Parallel()
	s := harvestFixtureDetail(t, string(defaults.HarnessClaudeCode), "claude-fixture", "", "6a46bcd8-b365-4608-be7d-5601e2fdd805")
	if len(s.Turns) != 4 {
		t.Errorf("detail turns = %d, want the fixture's 4 records", len(s.Turns))
	}
	for i, turn := range s.Turns {
		if turn.Timestamp.IsZero() {
			t.Errorf("turn %d (%s, depth %d) has no timestamp: %q", i, turn.Role, turn.Depth, turn.Content)
		}
		if i > 0 && turn.Content == s.Turns[i-1].Content {
			t.Errorf("turn %d repeats turn %d: %q", i, i-1, turn.Content)
		}
	}
}
