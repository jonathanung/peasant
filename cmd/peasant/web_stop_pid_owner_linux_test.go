//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestStopWebLeavesARenamedProgramAlone is the #714 re-review repro: another
// program copied under this executable's file name and started with the
// server's arguments must not be signalled. On Linux /proc/<pid>/exe gives the
// exact path, so a file-name match alone is not enough.
func TestStopWebLeavesARenamedProgramAlone(t *testing.T) {
	t.Parallel()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(python)
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(t.TempDir(), filepath.Base(self))
	if err := os.WriteFile(renamed, data, 0o755); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	cmd := exec.Command(renamed, "-c", "import time; time.sleep(300)", "web", "start", "--foreground", "--port", strconv.Itoa(port))
	exited := startChild(t, cmd)
	writePIDFile(t, port, cmd.Process.Pid)
	deadline := time.Now().Add(10 * time.Second)
	for !commandLineShows(cmd.Process.Pid, "time.sleep") {
		if time.Now().After(deadline) {
			t.Fatal("renamed python3 stand-in never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	assertNotSignalled(t, "python3 renamed to this executable's name", cmd, exited, stopWeb(port))
}
