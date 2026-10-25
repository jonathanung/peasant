package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/peasant-labs/peasant/internal/api"
	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/peasant/internal/sessionvisibility"
	"github.com/peasant-labs/schema"
)

// harvestFixtureDetail harvests one of the repo's e2e fixtures with only its
// harness enabled and returns the session detail GET /api/v1/sessions/{id}
// serves for id.
func harvestFixtureDetail(t *testing.T, harness, fixture, sourceSubdir, id string) *schema.SessionDetailReadPayload {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "src")
	if err := os.CopyFS(src, os.DirFS(filepath.Join("..", "..", "internal", "e2e", "testdata", fixture))); err != nil {
		t.Fatal(err)
	}
	sources := ""
	for _, h := range []defaults.Harness{defaults.HarnessClaudeCode, defaults.HarnessOpenCode, defaults.HarnessCursor, defaults.HarnessCodex, defaults.HarnessStrike, defaults.HarnessPi} {
		if string(h) == harness {
			sources += fmt.Sprintf("  %s:\n    enabled: true\n    paths:\n      - %s\n", h, filepath.Join(src, sourceSubdir))
		} else {
			sources += fmt.Sprintf("  %s: {enabled: false}\n", h)
		}
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\nsources:\n"+sources+"output:\n  basePath: "+filepath.Join(dir, "out")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := executeHarvestCmd(t, dir, []string{"--config=" + configPath}); err != nil {
		t.Fatalf("harvest %s: %v\n%s", fixture, err, out)
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := sessionvisibility.New(cfg.Selection)
	if err != nil {
		t.Fatal(err)
	}
	options, err := generationStoreOptions(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := openPreparedStore(t, string(defaults.ResolveDBFilePathWith(dir)), options...)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	session, err := api.SessionDetailReadForProvider(t.Context(), api.NewStoreDataProvider(db, policy, filepath.Join(dir, "out")), id)
	if err != nil {
		t.Fatalf("session detail %s: %v", id, err)
	}
	return session
}
