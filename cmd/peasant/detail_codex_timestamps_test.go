package main

import (
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
)

// TestSessionDetailCarriesCodexTimestamps guards #683: every Codex rollout
// line carries a timestamp, but the detail API served each Codex turn as
// 0001-01-01 because the classifier never stamped its blocks.
func TestSessionDetailCarriesCodexTimestamps(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"019e7724-38f5-7660-b2d7-2eda237d4a7d", "019e7724-8abb-7851-adac-69a30b4bc1bb"} {
		s := harvestFixtureDetail(t, string(defaults.HarnessCodex), "codex-fixture", "sessions", id)
		if len(s.Turns) == 0 {
			t.Fatalf("session %s: no turns", id)
		}
		for i, turn := range s.Turns {
			if turn.Timestamp.Year() != 2024 {
				t.Errorf("session %s turn %d (%s): timestamp %v, want the rollout's 2024-01-02 line timestamp", id, i, turn.Role, turn.Timestamp)
			}
		}
	}
}
