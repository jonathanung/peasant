package main

import (
	"os"
	"path/filepath"
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

// TestSessionDetailKeepsTwoIdenticalClaudeBlocks is the #726 review repro
// through the real harvest and detail read: an assistant message with two
// real "ok" text blocks must keep both. The first version of the fold dropped
// one of them; that failed this test at runtime.
func TestSessionDetailKeepsTwoIdenticalClaudeBlocks(t *testing.T) {
	t.Parallel()
	const sid = "6a46bcd8-0000-4000-8000-000000000726"
	fixture := t.TempDir()
	lines := `{"type":"user","sessionId":"` + sid + `","uuid":"u1","cwd":"/tmp/qa-726","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"say ok twice"}}
{"type":"assistant","sessionId":"` + sid + `","uuid":"a1","parentUuid":"u1","cwd":"/tmp/qa-726","timestamp":"2026-10-01T10:00:01Z","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"},{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":2}}}
`
	if err := os.MkdirAll(filepath.Join(fixture, "-tmp-qa-726"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "-tmp-qa-726", sid+".jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	s := harvestFixtureDetail(t, string(defaults.HarnessClaudeCode), fixture, "", sid)
	blocks := 0
	for _, turn := range s.Turns {
		if turn.Depth == 1 && turn.Content == "ok" {
			blocks++
		}
	}
	if blocks != 2 {
		t.Errorf("detail shows %d of the message's 2 real \"ok\" blocks; turns: %+v", blocks, s.Turns)
	}
}
