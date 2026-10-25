package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peasant-labs/peasant/internal/ingest"
	"github.com/peasant-labs/peasant/internal/salt"
	"github.com/peasant-labs/peasant/internal/testutil"
)

// TestDiscoveryReportsUnreadableSourceDirectories proves Claude and Codex
// discovery name a directory they could not read instead of silently
// reporting fewer sessions.
func TestDiscoveryReportsUnreadableSourceDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not deny directory access on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode-000 directory")
	}
	fs := &ingest.OSFileSystem{}
	cases := map[string]func() ingest.SourceAdapter{
		string(ingest.HarnessClaudeCode): func() ingest.SourceAdapter {
			return ingest.NewClaudeAdapter(fs, testutil.DefaultGitResolver(), salt.Salt{})
		},
		string(ingest.HarnessCodex): func() ingest.SourceAdapter {
			return ingest.NewCodexAdapter(fs, testutil.DefaultGitResolver(), salt.Salt{})
		},
	}
	for name, newAdapter := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			locked := filepath.Join(root, "locked")
			if err := os.Mkdir(locked, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

			adapter := newAdapter()
			if _, err := adapter.Discover(context.Background(), ingest.SourceConfig{Paths: []ingest.ResolvedPath{ingest.ResolvedPath(root)}}); err != nil {
				t.Fatalf("discover: %v", err)
			}
			reporter, ok := adapter.(ingest.DiscoveryDiagnosticReporter)
			if !ok {
				t.Fatalf("%s adapter reports no discovery diagnostics", name)
			}
			diagnostics := reporter.DiscoveryDiagnostics()
			if len(diagnostics) != 1 || diagnostics[0].Location != locked || !strings.Contains(diagnostics[0].Detail, "permission denied") {
				t.Fatalf("want one permission diagnostic for %s, got %+v", locked, diagnostics)
			}
		})
	}
}
