package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarvestRefusesOneSessionIDInTwoSourceFiles proves two source files that
// claim one session id fail that session with a reason naming both files,
// instead of storing a row mixed from both and reporting 0 errors.
func TestHarvestRefusesOneSessionIDInTwoSourceFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "projects")
	sid := "ffffffff-0000-4000-8000-000000000001"
	turns := func(cwd string, n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, `{"type":"user","sessionId":"%s","uuid":"u%d-%s","cwd":"%s","timestamp":"2026-10-01T10:0%d:00Z","message":{"role":"user","content":"turn %d"}}`+"\n", sid, i, cwd, cwd, i, i)
		}
		return b.String()
	}
	for _, p := range []struct {
		project string
		n       int
	}{{"-tmp-qa-one", 3}, {"-tmp-qa-two", 7}} {
		if err := os.MkdirAll(filepath.Join(src, p.project), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, p.project, sid+".jsonl"), []byte(turns("/tmp/"+p.project, p.n)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := fmt.Sprintf("version: 1\nsources:\n  claude-code:\n    enabled: true\n    paths:\n      - %s\n  opencode: {enabled: false}\n  cursor: {enabled: false}\n  codex: {enabled: false}\n  strike: {enabled: false}\n  pi: {enabled: false}\noutput:\n  basePath: %s\n", src, filepath.Join(dir, "out"))
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := executeHarvestCmd(t, dir, []string{"--config=" + configPath})
	if err == nil {
		t.Fatalf("harvest succeeded although two source files claim session %s:\n%s", sid, output)
	}
	if !strings.Contains(output, "   - 1     errors") || !strings.Contains(output, "same session id") ||
		!strings.Contains(output, "-tmp-qa-one") || !strings.Contains(output, "-tmp-qa-two") {
		t.Fatalf("harvest does not report the duplicate id as one failed session naming both files:\n%s", output)
	}
}
