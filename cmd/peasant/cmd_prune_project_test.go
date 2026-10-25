package main

import (
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/peasant/third_party/zombiezen-sqlite/sqlitex"
)

// TestPruneProjectAcceptsSessionsListIdentifiers runs the real prune command
// with every project identifier sessions list --project takes (name, path,
// remote in its canonical and URL forms) plus the hash, and checks that a
// near-miss name matches nothing.
func TestPruneProjectAcceptsSessionsListIdentifiers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneTestSessions(t, dir)
	db, err := openPreparedStore(t, string(defaults.ResolveDBFilePathWith(dir)))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Pool().Take(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	err = sqlitex.ExecuteTransient(conn, "UPDATE projects SET canonical_remote = 'github.com/example/prune-test' WHERE project_hash = ?",
		&sqlitex.ExecOptions{Args: []any{pruneTestProject}})
	db.Pool().Put(conn)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{
		pruneTestProject,
		"prune-test",
		"PRUNE-TEST",
		"prune", // the directory name of /test/prune
		"/test/prune",
		"github.com/example/prune-test",
		"https://github.com/example/prune-test.git",
		"git@github.com:example/prune-test.git",
	} {
		out, _, err := executePruneCmd(t, dir, []string{"--project", id, "--dry-run"})
		if err != nil {
			t.Errorf("prune --project %q: %v", id, err)
		} else if !strings.Contains(out, "2 session(s) would be deleted") {
			t.Errorf("prune --project %q: want both sessions; got:\n%s", id, out)
		}
	}
	for _, id := range []string{"rune", "prune-tes", "example", "/test", "example/prune"} {
		out, _, err := executePruneCmd(t, dir, []string{"--project", id, "--dry-run"})
		if err != nil {
			t.Errorf("prune --project %q: %v", id, err)
		} else if strings.Contains(out, "would be deleted") {
			t.Errorf("prune --project %q matched a project by a partial name; got:\n%s", id, out)
		}
	}
}
