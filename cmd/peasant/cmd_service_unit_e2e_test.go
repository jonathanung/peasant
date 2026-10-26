package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unitRun is what running the service unit produced.
type unitRun struct {
	harvestExit int
	redactRan   bool
	redactExit  int
	output      string
}

// failed is the unit's result: systemd reports a oneshot failed when its
// ExecStart= failed, whatever ExecStopPost= returned.
func (r unitRun) failed() bool { return r.harvestExit != 0 }

// systemdEnvValue unquotes one Environment= value: C escapes \\ and \"
// inside double quotes and %% for a literal %.
func systemdEnvValue(t *testing.T, quoted string) string {
	t.Helper()
	if len(quoted) < 2 || quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
		t.Fatalf("Environment= value %q is not double-quoted", quoted)
	}
	var b strings.Builder
	body := quoted[1 : len(quoted)-1]
	for i := 0; i < len(body); i++ {
		switch {
		case body[i] == '\\':
			i++
			b.WriteByte(body[i])
		case body[i] == '%':
			i++
			b.WriteByte('%')
		default:
			b.WriteByte(body[i])
		}
	}
	return b.String()
}

// runServiceUnit runs peasant-harvest.service the way systemd runs a
// oneshot: ExecStart=, then every ExecStopPost= whether ExecStart= failed or
// not, each in a subprocess whose environment is only HOME, PATH and the
// unit's Environment= lines (the user manager's environment, not the test's).
// The unit names the real peasant binary, so this is the production path.
func runServiceUnit(t *testing.T, unitText, home string) unitRun {
	t.Helper()
	env := []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	for _, value := range unitDirectiveValues(unitText, "Environment=") {
		env = append(env, systemdEnvValue(t, value))
	}
	var out bytes.Buffer
	exec1 := func(line string) int {
		words := splitSystemdExec(t, line)
		cmd := exec.Command(words[0], words[1:]...)
		cmd.Env = env
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		if err != nil {
			t.Fatalf("start %q: %v", words[0], err)
		}
		return 0
	}
	starts := execStartsOf(unitText)
	if len(starts) != 1 {
		t.Fatalf("unit has %d ExecStart= lines, want 1", len(starts))
	}
	run := unitRun{harvestExit: exec1(starts[0])}
	for _, line := range unitDirectiveValues(unitText, "ExecStopPost=") {
		run.redactRan = true
		run.redactExit = exec1(line)
	}
	run.output = out.String()
	return run
}

// installServiceForE2E installs the unit for the real peasant binary into a
// fresh HOME with a Claude Code source holding one seeded-token session.
func installServiceForE2E(t *testing.T, configYAML string) (*serviceHarness, string, string) {
	t.Helper()
	return installServiceForE2EIn(t, newServiceHarness(t, "home dir"), configYAML)
}

// transcriptCopies lists the peasant-sync transcript copies under root.
func transcriptCopies(t *testing.T, root string) []string {
	t.Helper()
	var copies []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, "--transcript.jsonl") {
			copies = append(copies, path)
		}
		return err
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return copies
}

func assertCopiesRedacted(t *testing.T, root string) {
	t.Helper()
	copies := transcriptCopies(t, root)
	if len(copies) == 0 {
		t.Fatalf("no transcript copy under %s", root)
	}
	for _, path := range copies {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(serviceSeededToken)) {
			t.Errorf("%s still holds the seeded token", path)
		}
	}
}

// TestServiceUnitRedactsWhenHarvestFails: one corrupt session makes harvest
// exit 1. The copies harvest did write must still be redacted, and the unit
// must still report failure.
func TestServiceUnitRedactsWhenHarvestFails(t *testing.T) {
	t.Parallel()
	h, home, src := installServiceForE2E(t, "")
	bad := filepath.Join(src, "-svc-demo", "22222222-2222-4222-8222-222222222222.jsonl")
	if err := os.WriteFile(bad, []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := runServiceUnit(t, h.read(t, serviceUnitName), home)
	if run.harvestExit == 0 {
		t.Fatalf("harvest succeeded with a corrupt session; the test needs it to fail:\n%s", run.output)
	}
	if !run.redactRan || run.redactExit != 0 || !strings.Contains(run.output, "redacted 1 session(s)") {
		t.Fatalf("redact after a failed harvest: ran=%v exit=%d\n%s", run.redactRan, run.redactExit, run.output)
	}
	if !run.failed() {
		t.Error("the unit reported success after harvest failed")
	}
	assertCopiesRedacted(t, filepath.Join(home, "peasant", "peasant-sync"))
}

// TestServiceUnitHarvestAndRedactAgreeThroughUnitEnv: no --output anywhere.
// Harvest and redact must find the same copies through the unit's own
// Environment= and directory flags, with the default and a custom basePath.
func TestServiceUnitHarvestAndRedactAgreeThroughUnitEnv(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		basePath func(home string) string
	}{
		{"default peasant-sync under the data dir", nil},
		{"custom output.basePath", func(home string) string { return filepath.Join(home, "custom out", "sync") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newServiceHarness(t, "home dir")
			home := h.dir
			sync := filepath.Join(home, "peasant", "peasant-sync")
			config := ""
			if tc.basePath != nil {
				sync = tc.basePath(home)
				config = "output:\n  basePath: " + sync + "\n"
			}
			h, home, _ = installServiceForE2EIn(t, h, config)
			run := runServiceUnit(t, h.read(t, serviceUnitName), home)
			if run.failed() || run.redactExit != 0 {
				t.Fatalf("unit run failed (harvest %d, redact %d):\n%s", run.harvestExit, run.redactExit, run.output)
			}
			if !strings.Contains(run.output, "redacted 1 session(s)") {
				t.Fatalf("redact did not find the harvested copy:\n%s", run.output)
			}
			assertCopiesRedacted(t, sync)
		})
	}
}

// installServiceForE2EIn is installServiceForE2E on an existing harness.
func installServiceForE2EIn(t *testing.T, h *serviceHarness, configYAML string) (*serviceHarness, string, string) {
	t.Helper()
	h.exe = buildPeasantMatrixBinary(t)
	home := h.dir
	src := filepath.Join(home, ".claude", "projects")
	writeServiceClaudeSession(t, src, "demo")
	if configYAML != "" {
		cfgDir := filepath.Join(home, "peasant")
		if err := os.MkdirAll(cfgDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(configYAML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := h.run(t, "install"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	return h, home, src
}
