package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarvestReportsSessionsExcludedBySelection proves sessions the saved
// selection leaves out are reported as excluded, with a hint, instead of being
// counted "unchanged" on a data dir that has never stored them.
func TestHarvestReportsSessionsExcludedBySelection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "projects")
	for i := 1; i <= 2; i++ {
		sid := fmt.Sprintf("bbbbbbbb-0000-4000-8000-00000000000%d", i)
		line := `{"type":"user","sessionId":"` + sid + `","uuid":"u1","cwd":"/tmp/qa-sel","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n"
		if err := os.MkdirAll(filepath.Join(src, "-tmp-qa-sel"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, "-tmp-qa-sel", sid+".jsonl"), []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The selection kickstart saves on a machine with no projects yet.
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := fmt.Sprintf("version: 1\nsources:\n  claude-code:\n    enabled: true\n    paths:\n      - %s\n  opencode: {enabled: false}\n  cursor: {enabled: false}\n  codex: {enabled: false}\n  strike: {enabled: false}\n  pi: {enabled: false}\noutput:\n  basePath: %s\nselection:\n  mode: selected\n", src, filepath.Join(dir, "out"))
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--config=" + configPath, "--dry-run"}, {"--config=" + configPath}} {
		output, err := executeHarvestCmd(t, dir, args)
		if err != nil {
			t.Fatalf("harvest %v: %v\n%s", args, err, output)
		}
		if !strings.Contains(output, "   - 0     unchanged") {
			t.Errorf("harvest %v counted unselected sessions as unchanged:\n%s", args, output)
		}
		if !strings.Contains(output, "   - 2     excluded by selection") || !strings.Contains(output, "peasant config") {
			t.Errorf("harvest %v did not report the 2 sessions excluded by selection with a hint:\n%s", args, output)
		}
	}
}
