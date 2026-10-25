package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarvestHumanOutputNamesWhyASessionFailed proves the human summary says
// why a session failed instead of only "ERROR <harness> <id>".
func TestHarvestHumanOutputNamesWhyASessionFailed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "projects")
	sid := "dddddddd-0000-4000-8000-000000000001"
	rec := func(uuid string) string {
		return `{"type":"user","sessionId":"` + sid + `","uuid":"` + uuid + `","cwd":"/tmp/qa-reason","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n"
	}
	body := rec("u1") + "{\"type\":\"user\",\"broken\n" + rec("u3")
	if err := os.MkdirAll(filepath.Join(src, "-tmp-qa-reason"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "-tmp-qa-reason", sid+".jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := fmt.Sprintf("version: 1\nsources:\n  claude-code:\n    enabled: true\n    paths:\n      - %s\n  opencode: {enabled: false}\n  cursor: {enabled: false}\n  codex: {enabled: false}\n  strike: {enabled: false}\n  pi: {enabled: false}\noutput:\n  basePath: %s\n", src, filepath.Join(dir, "out"))
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--config=" + configPath}, {"--config=" + configPath, "--verbose"}} {
		output, err := executeHarvestCmd(t, dir, args)
		if err == nil {
			t.Fatalf("harvest %v should fail the malformed session:\n%s", args, output)
		}
		if !strings.Contains(output, "ERROR") || !strings.Contains(output, "capture JSONL record 2: malformed complete JSON") {
			t.Errorf("harvest %v does not say why the session failed:\n%s", args, output)
		}
	}
}
