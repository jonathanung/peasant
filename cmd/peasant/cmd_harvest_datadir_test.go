package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peasant-labs/peasant/internal/config"
	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/spf13/cobra"
)

// TestHarvestDataDirOwnsPeasantSync proves --data-dir moves the peasant-sync
// transcript copies with the database, as its help text promises, instead of
// writing them under the default XDG data home.
func TestHarvestDataDirOwnsPeasantSync(t *testing.T) {
	// t.Setenv (not t.Parallel) on purpose: the default output path is
	// derived from XDG_DATA_HOME, and the assertion that nothing lands there
	// needs a default data home no parallel test writes to. TestMain's shared
	// throwaway XDG dir cannot give that guarantee.
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

// TestRuntimeConfigFollowsDataDir proves every config loader a running
// command reads paths from applies --data-dir once: the push/harvest run
// loader (dry and real) and the runtime loader, which is also the loader
// runWebForeground builds the config for its publish and reindex routes with. A configured
// output.basePath elsewhere keeps winning, and the loader behind config
// editing still returns the file's own value so a save never writes the
// override into config.yaml.
func TestRuntimeConfigFollowsDataDir(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	want := filepath.Join(string(defaults.ResolveDataDirPathWith(dataDir)), defaults.OutputSyncSubdir)
	newCmd := func() *cobra.Command {
		root := newTestRoot()
		sub := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
		root.AddCommand(sub)
		root.SetArgs([]string{"--data-dir", dataDir, "probe"})
		root.SetOut(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		return sub
	}
	defaultConfig := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(defaultConfig, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "elsewhere")
	customConfig := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(customConfig, []byte("version: 1\noutput:\n  basePath: "+custom+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newCmd()
	for name, load := range map[string]func(string) (string, error){
		"run loader":     func(p string) (string, error) { c, err := loadRunConfig(cmd, p, false); return basePathOf(c), err },
		"dry run loader": func(p string) (string, error) { c, err := loadRunConfig(cmd, p, true); return basePathOf(c), err },
		"runtime loader": func(p string) (string, error) { c, err := loadCommandConfig(cmd, p); return basePathOf(c), err },
	} {
		if got, err := load(defaultConfig); err != nil || got != want {
			t.Errorf("%s: basePath = %q (err %v), want %q", name, got, err, want)
		}
		if got, err := load(customConfig); err != nil || got != custom {
			t.Errorf("%s with a configured basePath: got %q (err %v), want %q", name, got, err, custom)
		}
	}
	edited, err := loadConfig(defaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Output.BasePath == want {
		t.Errorf("the editing loader applied --data-dir; a config save would persist it")
	}
}

func basePathOf(c *config.Config) string {
	if c == nil {
		return ""
	}
	return c.Output.BasePath
}

// TestHarvestDataDirUpgradeNeedsForceToCopy pins the documented upgrade
// path from a build that kept the database under --data-dir but wrote the
// peasant-sync copies under the default data home. The database row is
// current, so the first plain harvest classifies the session unchanged and
// writes no copy under <data-dir>/peasant/peasant-sync: classification does
// not stat the managed tree for unchanged sessions (follow-up: #757).
// The CHANGELOG remedy, one `harvest --force`, must copy it.
func TestHarvestDataDirUpgradeNeedsForceToCopy(t *testing.T) {
	xdgData := t.TempDir()
	t.Setenv(defaults.EnvXDGDataHome.String(), xdgData)
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "projects")
	sid := "aaaaaaaa-0000-4000-8000-000000000002"
	transcript := `{"type":"user","sessionId":"` + sid + `","uuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"hello"}}
{"type":"assistant","sessionId":"` + sid + `","uuid":"a1","parentUuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:05Z","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":3,"output_tokens":2}}}
`
	if err := os.MkdirAll(filepath.Join(src, "-tmp-qa-proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "-tmp-qa-proj", sid+".jsonl"), []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	base := "version: 1\nsources:\n  claude-code:\n    enabled: false\n  opencode:\n    enabled: false\n  cursor:\n    enabled: false\n  pi:\n    enabled: false\n"
	oldSync := filepath.Join(xdgData, "peasant", "peasant-sync")
	harvest := func(name, cfg string, extra ...string) string {
		t.Helper()
		configPath := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
		root := newTestRoot()
		root.AddCommand(BuildHarvestCommand())
		var out strings.Builder
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append([]string{"--data-dir", dir, "--config-dir", dir, "--config", configPath, "harvest", "--source-harness", "claude-code", "--source-path", src}, extra...))
		if err := root.Execute(); err != nil {
			t.Fatalf("%s harvest: %v\n%s", name, err, out.String())
		}
		return out.String()
	}
	// Old enough that neither harvest treats the session as active.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(src, "-tmp-qa-proj", sid+".jsonl"), old, old); err != nil {
		t.Fatal(err)
	}
	// The trailing separator keeps this build's override from moving the
	// pre-upgrade run, so it writes where the old builds wrote.
	pre := harvest("pre-upgrade", base+"output:\n  basePath: "+oldSync+string(filepath.Separator)+"\n")
	if entries, _ := os.ReadDir(oldSync); len(entries) == 0 {
		t.Fatalf("setup: the pre-upgrade harvest wrote no copies under %s:\n%s", oldSync, pre)
	}
	want := filepath.Join(string(defaults.ResolveDataDirPathWith(dir)), "peasant-sync")
	copies := func() int {
		n := 0
		_ = filepath.WalkDir(want, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.Contains(p, sid) {
				n++
			}
			return nil
		})
		return n
	}
	out := harvest("post-upgrade", base)
	if !strings.Contains(out, "1     unchanged") || copies() != 0 {
		t.Errorf("first harvest after the upgrade: want the session unchanged and no copy under %s (the documented limit); got %d copies:\n%s", want, copies(), out)
	}
	out = harvest("post-upgrade --force", base, "--force")
	if copies() == 0 {
		t.Errorf("harvest --force after the upgrade left no copy of %s under %s:\n%s", sid, want, out)
	}
}
