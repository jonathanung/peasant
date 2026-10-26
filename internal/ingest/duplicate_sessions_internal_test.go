package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRefuseDuplicateSessionIDsScope proves only file-per-session harnesses
// are refused, a deselected duplicate is left to the selection, and the
// refusal names the newer and larger file and says not to delete either.
func TestRefuseDuplicateSessionIDsScope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	small, big := filepath.Join(dir, "a.jsonl"), filepath.Join(dir, "b.jsonl")
	if err := os.WriteFile(small, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(big, []byte("xxxxxxx"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	pair := func(h Harness) []DiscoveredSession {
		return []DiscoveredSession{
			{Harness: h, SessionID: "s1", SourcePath: ResolvedPath(small), ModTime: now},
			{Harness: h, SessionID: "s1", SourcePath: ResolvedPath(big), ModTime: now.Add(-time.Hour)},
		}
	}
	for _, c := range []struct {
		name     string
		harness  Harness
		selected func(DiscoveredSession) bool
		refused  bool
	}{
		{"claude-two-files", HarnessClaudeCode, nil, true},
		{"codex-two-files", HarnessCodex, nil, true},
		{"opencode-two-stores", HarnessOpenCode, nil, false},
		{"cursor-two-paths", HarnessCursor, nil, false},
		{"claude-deselected", HarnessClaudeCode, func(DiscoveredSession) bool { return false }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			kept, refused := refuseDuplicateSessionIDs(&OSFileSystem{}, pair(c.harness), c.selected)
			if got := len(refused) == 1; got != c.refused {
				t.Fatalf("refused = %v, want %v", refused, c.refused)
			}
			if !c.refused {
				if len(kept) != 2 {
					t.Fatalf("kept %d sessions, want both", len(kept))
				}
				return
			}
			msg := refused[0].Error.Error()
			for _, want := range []string{"newer: " + small, "larger: " + big, "do not delete"} {
				if !strings.Contains(msg, want) {
					t.Fatalf("message %q lacks %q", msg, want)
				}
			}
		})
	}
}

// TestRefuseDuplicateSessionIDsLeavesOutChildrenOfARefusedParent proves a
// subagent of a refused session is not kept to be stored on its own.
func TestRefuseDuplicateSessionIDsLeavesOutChildrenOfARefusedParent(t *testing.T) {
	t.Parallel()
	parent := SessionID("s1")
	sessions := []DiscoveredSession{
		{Harness: HarnessClaudeCode, SessionID: parent, SourcePath: "/a/s1.jsonl"},
		{Harness: HarnessClaudeCode, SessionID: parent, SourcePath: "/b/s1.jsonl"},
		{Harness: HarnessClaudeCode, SessionID: "child", ParentUUID: &parent, SourcePath: "/a/s1/subagents/agent-child.jsonl"},
		{Harness: HarnessClaudeCode, SessionID: "other", SourcePath: "/a/other.jsonl"},
	}
	kept, refused := refuseDuplicateSessionIDs(nil, sessions, nil)
	if len(refused) != 1 || len(kept) != 1 || kept[0].SessionID != "other" {
		t.Fatalf("kept %+v refused %d, want only the unrelated session kept and one refusal", kept, len(refused))
	}
}
