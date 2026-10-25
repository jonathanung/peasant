package ingest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/ingest"
	"github.com/peasant-labs/peasant/internal/salt"
	"github.com/peasant-labs/peasant/internal/testutil"
)

// TestClaudeAdapter_DiscoveryTitleAgreesWithIndexerOnIsMeta proves the
// discovery hints skip an isMeta record as a title candidate, still take the
// branch and CWD from it, and pick the same record the indexer keeps as the
// first user entry.
func TestClaudeAdapter_DiscoveryTitleAgreesWithIndexerOnIsMeta(t *testing.T) {
	const sid = "11111111-1111-4111-8111-111111111111"
	lines := []string{
		`{"sessionId":"` + sid + `","type":"user","uuid":"u1","cwd":"/workspace/meta","gitBranch":"meta-branch","isMeta":true,"message":{"role":"user","content":"Your claude.ai usage limit has reset. Continue the task you were working on."}}`,
		`{"sessionId":"` + sid + `","type":"user","uuid":"u2","parentUuid":"u1","message":{"role":"user","content":"Profile the generation validator"}}`,
	}
	fs := testutil.NewMemFS()
	path := "/claude/-workspace/" + sid + ".jsonl"
	if err := fs.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	adapter := ingest.NewClaudeAdapter(fs, testutil.DefaultGitResolver(), salt.Salt{})
	sessions, err := adapter.Discover(context.Background(), ingest.SourceConfig{Paths: []ingest.ResolvedPath{"/claude"}, Enabled: true})
	if err != nil || len(sessions) != 1 {
		t.Fatalf("discover: sessions=%d err=%v", len(sessions), err)
	}
	got := sessions[0]
	if got.Branch != "meta-branch" || got.CWD != "/workspace/meta" {
		t.Fatalf("branch/cwd = %q/%q, want hints from the isMeta record", got.Branch, got.CWD)
	}

	entries, err := ingest.NewClaudeIndexer(fs).IndexTranscript(context.Background(), got)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	var firstUser string
	for _, e := range entries {
		if e.Role == ingest.RoleUser && e.ContentPreview != nil {
			firstUser = *e.ContentPreview
			break
		}
	}
	if firstUser == "" {
		t.Fatalf("indexer kept no user entry")
	}
	if got.Title != firstUser {
		t.Fatalf("discovery title %q disagrees with the indexer's first user entry %q", got.Title, firstUser)
	}
}
