package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyContentFailsOnADamagedSyncedTranscript guards #682: a saved
// transcript under peasant-sync/ that was deleted or truncated must fail
// verify --content and name the session, and the printed recovery heals it.
func TestVerifyContentFailsOnADamagedSyncedTranscript(t *testing.T) {
	t.Parallel()
	for name, damage := range map[string]func(path string) error{
		"deleted":   os.Remove,
		"truncated": func(path string) error { return os.Truncate(path, 40) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			seed := seedVerifyHarvest(t)
			syncDir := filepath.Join(seed.dir, "out") // output.basePath in the seed config
			matches, err := filepath.Glob(filepath.Join(syncDir, "*", seed.sessionID, seed.sessionID+"--transcript.jsonl"))
			if err != nil || len(matches) != 1 {
				t.Fatalf("find the saved transcript under %s: %v %v", syncDir, matches, err)
			}
			if err := damage(matches[0]); err != nil {
				t.Fatal(err)
			}

			out, err := seed.run(t, "verify", "--content")
			if err == nil || strings.Contains(out, "Status: PASSED") {
				t.Errorf("verify --content with the saved transcript %s: err = %v, want a failure and no PASSED:\n%s", name, err, out)
			}
			if !strings.Contains(out, "saved transcripts damaged or missing: 1") || !strings.Contains(out, seed.sessionID) {
				t.Errorf("verify --content did not name the session with the %s transcript:\n%s", name, out)
			}

			if out, err := seed.run(t, "--force", "--session", seed.sessionID); err != nil {
				t.Fatalf("recovery harvest: %v\n%s", err, out)
			}
			if out, err := seed.run(t, "verify", "--content"); err != nil || !strings.Contains(out, "Status: PASSED") {
				t.Errorf("verify --content after the printed recovery: err = %v, want PASSED:\n%s", err, out)
			}
		})
	}
}
