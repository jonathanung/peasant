package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/peasant-labs/peasant/internal/tui/ftue"
	"github.com/peasant-labs/peasant/internal/tui/kickstart"
)

// TestInteractiveCommandsWithoutTerminal runs kickstart and config through
// their production Bubble Tea boundaries with no terminal reachable. Each must
// fail with one actionable line: no raw Bubble Tea error and no usage block.
// Not parallel: it swaps the package-level terminal probe.
func TestInteractiveCommandsWithoutTerminal(t *testing.T) {
	saved := terminalAvailable
	terminalAvailable = func() bool { return false }
	t.Cleanup(func() { terminalAvailable = saved })

	kick := defaultKickstartCommandDeps()
	kick.discover = func(context.Context, string, string, *discoverySpinner) (ftue.ProviderInventory, []ftue.SessionListing, kickstart.SubagentRelation) {
		return ftue.ProviderInventory{}, nil, nil
	}
	conf := defaultConfigCommandDeps()
	conf.discover = func(context.Context, string, string) configDiscovery {
		return configDiscovery{source: newKickstartScannerSource(nil, nil, nil, nil, nil)}
	}

	for name, build := range map[string]func() *cobra.Command{
		"kickstart": func() *cobra.Command { return buildKickstartCommand(kick) },
		"config":    func() *cobra.Command { return buildConfigCommand(conf) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			root := newTestRoot()
			root.AddCommand(build())
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetIn(&bytes.Buffer{})
			root.SetArgs([]string{"--data-dir", dir, "--config-dir", dir, "--state-dir", dir, name})
			err := root.Execute()
			if err == nil {
				t.Fatalf("peasant %s without a terminal succeeded", name)
			}
			if !strings.Contains(err.Error(), "needs an interactive terminal") || strings.Contains(err.Error(), "bubbletea") {
				t.Errorf("error = %q, want the one-line terminal message without the raw Bubble Tea error", err)
			}
			if strings.Contains(out.String(), "Usage:") {
				t.Errorf("usage block printed for a missing terminal:\n%s", out.String())
			}
		})
	}
}
