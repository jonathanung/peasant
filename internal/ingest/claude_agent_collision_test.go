package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/ingest"
	"github.com/peasant-labs/peasant/internal/salt"
	"github.com/peasant-labs/peasant/internal/testutil"
)

// TestClaudeDiscoveryReportsAgentIDCollisionAcrossParents proves a subagent
// left out because another parent's subagent has the same agent id is
// reported, instead of disappearing without a word.
func TestClaudeDiscoveryReportsAgentIDCollisionAcrossParents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	record := func(sid, uuid, role, extra string) string {
		content := `"hello"`
		if role == "assistant" {
			content = `[{"type":"text","text":"ok"}]`
		}
		return `{"type":"` + role + `","sessionId":"` + sid + `","uuid":"` + uuid + `","cwd":"/tmp/qa"` + extra + `,"timestamp":"2026-10-01T10:00:00Z","message":{"role":"` + role + `","content":` + content + `}}` + "\n"
	}
	parents := []struct{ project, sid string }{
		{"-tmp-qa-alpha", "11111111-0000-4000-8000-000000000001"},
		{"-tmp-qa-delta", "22222222-0000-4000-8000-000000000002"},
	}
	for _, p := range parents {
		write(filepath.Join(root, p.project, p.sid+".jsonl"), record(p.sid, "u1-"+p.sid, "user", "")+record(p.sid, "a1-"+p.sid, "assistant", ""))
		sub := `,"isSidechain":true,"agentId":"a1b2c3d"`
		write(filepath.Join(root, p.project, p.sid, "subagents", "agent-a1b2c3d.jsonl"), record(p.sid, "su-"+p.sid, "user", sub)+record(p.sid, "sa-"+p.sid, "assistant", sub))
	}
	adapter := ingest.NewClaudeAdapter(&ingest.OSFileSystem{}, testutil.DefaultGitResolver(), salt.Salt{})
	sessions, err := adapter.Discover(context.Background(), ingest.SourceConfig{Enabled: true, Paths: []ingest.ResolvedPath{ingest.ResolvedPath(root)}})
	if err != nil {
		t.Fatal(err)
	}
	subagents := 0
	for _, s := range sessions {
		if s.ParentUUID != nil {
			subagents++
		}
	}
	if subagents != 1 {
		t.Fatalf("precondition: want one admitted subagent for the colliding id, got %d", subagents)
	}
	var found bool
	for _, d := range adapter.DiscoveryDiagnostics() {
		if d.Code == ingest.DiscoveryCodeClaudeAgentIDCollision && strings.Contains(d.Location, "agent-a1b2c3d.jsonl") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the subagent left out by the agent-id collision was not reported: %+v", adapter.DiscoveryDiagnostics())
	}
}
