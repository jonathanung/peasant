package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
)

// TestHarvestDataDirOwnsPeasantSync proves --data-dir moves the peasant-sync
// transcript copies with the database, as its help text promises, instead of
// writing them under the default XDG data home.
func TestHarvestDataDirOwnsPeasantSync(t *testing.T) {
	xdgData := t.TempDir()
	t.Setenv(defaults.EnvXDGDataHome.String(), xdgData)
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "projects")
	sid := "aaaaaaaa-0000-4000-8000-000000000001"
	transcript := `{"type":"user","sessionId":"` + sid + `","uuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"hello"}}
{"type":"assistant","sessionId":"` + sid + `","uuid":"a1","parentUuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:05Z","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":3,"output_tokens":2}}}
`
	if err := os.MkdirAll(filepath.Join(src, "-tmp-qa-proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "-tmp-qa-proj", sid+".jsonl"), []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nsources:\n  claude-code:\n    enabled: false\n  opencode:\n    enabled: false\n  cursor:\n    enabled: false\n  pi:\n    enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := newTestRoot()
	root.AddCommand(BuildHarvestCommand())
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--data-dir", dir, "--config-dir", dir, "--config", configPath, "harvest", "--source-harness", "claude-code", "--source-path", src})
	if err := root.Execute(); err != nil {
		t.Fatalf("harvest: %v\n%s", err, out.String())
	}
	want := filepath.Join(string(defaults.ResolveDataDirPathWith(dir)), "peasant-sync")
	if !strings.Contains(out.String(), "Output:  "+want) {
		t.Errorf("harvest output root is not under --data-dir; want %s in:\n%s", want, out.String())
	}
	if _, err := os.Stat(filepath.Join(xdgData, "peasant", "peasant-sync")); !os.IsNotExist(err) {
		t.Errorf("harvest with --data-dir wrote peasant-sync under the default XDG data home (stat err %v)", err)
	}
	entries, _ := os.ReadDir(want)
	if len(entries) == 0 {
		t.Errorf("no transcript copies under %s", want)
	}
}
