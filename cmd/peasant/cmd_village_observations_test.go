package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/peasant-labs/peasant/internal/auth"
	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/peasant/internal/testutil"
	"github.com/peasant-labs/schema"
)

func TestVillageContributionCommandUsesSavedStore(t *testing.T) {
	f := testutil.LoadCollectiveEvidenceFixtures(t)
	for _, c := range f.Cases {
		if c.Mode != "traverse" {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			dir := t.TempDir()
			path := defaults.ResolveDBFilePathWith(dir).String()
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			db, err := openPreparedStore(t, path)
			if err != nil {
				t.Fatal(err)
			}
			creds := auth.Credentials{APIKey: "fixture-key", KeyID: "fixture-key-id", UserID: "11111111-1111-4111-8111-111111111111", VillageURL: "https://offline.invalid/installation", Username: "fixture"}
			if err := auth.SaveCredentialsFrom(&creds, dir); err != nil {
				t.Fatal(err)
			}
			snapshot, err := auth.CaptureCredentials(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			testutil.SeedCollectiveEvidence(t, t.Context(), db, snapshot, f)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			root := buildRootCommand()
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"--data-dir", dir, "--config-dir", dir, "--state-dir", dir, "village", "contributions", "list", "--kind", string(c.Kind), "--limit", "200", "--json"})
			if err := root.ExecuteContext(t.Context()); err != nil {
				t.Fatalf("mounted CLI failed: %v stderr=%s stdout=%s", err, &stderr, &stdout)
			}
			var response schema.LocalContributionHistoryResponse
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatalf("JSON stdout %q: %v", stdout.String(), err)
			}
			if response.Total != c.WantTotal || !reflect.DeepEqual(testutil.CollectiveEvidenceIDs(response), c.WantIDs) {
				t.Fatalf("CLI evidence=%#v want=%v", response, c.WantIDs)
			}
			if err := auth.ClearCredentialsFrom(dir); err != nil {
				t.Fatal(err)
			}
			root = buildRootCommand()
			stdout.Reset()
			stderr.Reset()
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"--data-dir", dir, "--config-dir", dir, "village", "collectives", "list", "--json"})
			if err := root.ExecuteContext(t.Context()); err == nil {
				t.Fatal("signed-out saved CLI used previous account cache")
			}
			if stdout.Len() != 0 {
				t.Fatalf("signed-out CLI leaked %s", &stdout)
			}
		})
	}
}
