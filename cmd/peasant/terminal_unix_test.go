//go:build unix

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
)

const noTerminalChildEnv = "PEASANT_TEST_NO_TERMINAL_CHILD"

// TestInteractiveCommandsWithoutTerminal runs kickstart, config and tui in a
// child process that has no controlling terminal (new session, stdin from
// /dev/null, stdout a pipe), the way cron, CI or `setsid … </dev/null` runs
// them. Each must exit with one actionable line: no raw Bubble Tea error and
// no usage block. The child drives the real terminal probe; nothing is faked.
func TestInteractiveCommandsWithoutTerminal(t *testing.T) {
	if args := os.Getenv(noTerminalChildEnv); args != "" {
		root := buildRootCommand()
		root.SetArgs(strings.Split(args, "\x1f"))
		if err := root.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "ERR:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	t.Parallel()
	for _, name := range []string{"kickstart", "config", "tui"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			dir := filepath.Join(home, "peasant")
			args := []string{"--data-dir", dir, "--config-dir", dir, "--state-dir", dir, name}
			child := exec.Command(os.Args[0], "-test.run=^TestInteractiveCommandsWithoutTerminal$")
			// A scratch HOME and XDG dirs, so discovery never reads the
			// developer's real transcripts.
			child.Env = append(os.Environ(),
				noTerminalChildEnv+"="+strings.Join(args, "\x1f"),
				"HOME="+home,
				defaults.EnvXDGConfigHome.String()+"="+filepath.Join(home, ".config"),
				defaults.EnvXDGDataHome.String()+"="+filepath.Join(home, ".local", "share"),
				defaults.EnvXDGStateHome.String()+"="+filepath.Join(home, ".local", "state"),
			)
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			var out bytes.Buffer
			child.Stdout = &out
			child.Stderr = &out
			err := child.Run()
			got := out.String()
			if err == nil {
				t.Fatalf("peasant %s without a terminal exited 0:\n%s", name, got)
			}
			if !strings.Contains(got, "needs an interactive terminal") || strings.Contains(strings.ToLower(got), "tty") {
				t.Errorf("want the one-line terminal message without the raw Bubble Tea TTY error, got:\n%s", got)
			}
			if strings.Contains(got, "Usage:") {
				t.Errorf("usage block printed for a missing terminal:\n%s", got)
			}
		})
	}
}
