package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/peasant/internal/tui/ftue"
	"github.com/peasant-labs/peasant/internal/tui/kickstart"
)

// seedResetFixture writes a config, a database, a peasant-sync transcript and a
// state file under isolated directories and returns their paths.
func seedResetFixture(t *testing.T) (dataDir, configDir string, paths []string) {
	t.Helper()
	dataDir, configDir = t.TempDir(), t.TempDir()
	t.Setenv(defaults.EnvXDGStateHome.String(), t.TempDir())
	paths = []string{
		defaults.ResolveConfigFilePathWith(configDir).String(),
		string(defaults.ResolveDBFilePathWith(dataDir)),
		filepath.Join(string(defaults.ResolveDataDirPathWith(dataDir)), "peasant-sync", "proj", "s1", "s1--transcript.jsonl"),
		filepath.Join(string(defaults.ResolveStateDirPath()), "web-18690.pid"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dataDir, configDir, paths
}

func runKickstartReset(t *testing.T, dataDir, configDir string, stdin *bytes.Buffer, args ...string) (error, bool) {
	t.Helper()
	deps := defaultKickstartCommandDeps()
	wizardReached := false
	deps.discover = func(context.Context, string, string, *discoverySpinner) (ftue.ProviderInventory, []ftue.SessionListing, kickstart.SubagentRelation) {
		wizardReached = true
		return ftue.ProviderInventory{}, nil, nil
	}
	deps.runFlow = func(*cobra.Command, kickstartCommandDeps, string, ftue.ProviderInventory, []ftue.SessionListing, kickstart.SubagentRelation) error {
		return nil
	}
	root := newTestRoot()
	root.AddCommand(buildKickstartCommand(deps))
	root.SetIn(stdin)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"--data-dir", dataDir, "--config-dir", configDir, "kickstart", "--reset"}, args...))
	return root.Execute(), wizardReached
}

// TestKickstartResetRefusesWithoutConsent proves --reset deletes nothing when
// its input is not a terminal and --confirm is absent.
func TestKickstartResetRefusesWithoutConsent(t *testing.T) {
	dataDir, configDir, paths := seedResetFixture(t)
	err, reached := runKickstartReset(t, dataDir, configDir, bytes.NewBufferString("y\n"))
	if err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("kickstart --reset without a terminal: err = %v, want a refusal naming --confirm", err)
	}
	if reached {
		t.Fatal("kickstart continued into discovery after refusing the reset")
	}
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("refused reset still removed %s: %v", p, statErr)
		}
	}
}

// TestKickstartResetWithConfirmRemovesEverything keeps the scripted path.
func TestKickstartResetWithConfirmRemovesEverything(t *testing.T) {
	dataDir, configDir, paths := seedResetFixture(t)
	if err, _ := runKickstartReset(t, dataDir, configDir, &bytes.Buffer{}, "--confirm"); err != nil {
		t.Fatalf("kickstart --reset --confirm: %v", err)
	}
	for _, p := range paths {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Fatalf("confirmed reset left %s (stat err %v)", p, statErr)
		}
	}
}
