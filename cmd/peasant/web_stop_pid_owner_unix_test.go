//go:build unix

package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const webServerStandInEnv = "PEASANT_TEST_WEB_SERVER_STAND_IN"

// freePort reserves and releases a loopback port, so nothing answers the
// HTTP shutdown request and `web stop` takes its PID-file fallback.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a loopback port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

// startChild starts cmd and returns a channel closed when it exits.
func startChild(t *testing.T, cmd *exec.Cmd) <-chan struct{} {
	t.Helper()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-exited })
	return exited
}

func writePIDFile(t *testing.T, port, pid int) string {
	t.Helper()
	path := pidFilePath(port)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

// TestStopWebLeavesAnUnrelatedProcessAlone guards #671: a stale PID file that
// names some other process (here `sleep`) must not get it signalled. The file
// is removed and the error says it was stale.
func TestStopWebLeavesAnUnrelatedProcessAlone(t *testing.T) {
	port := freePort(t)
	cmd := exec.Command("sleep", "300")
	exited := startChild(t, cmd)
	pidFile := writePIDFile(t, port, cmd.Process.Pid)

	err := stopWeb(port)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Errorf("stopWeb with a PID file naming `sleep` = %v, want a stale-PID-file error", err)
	}
	select {
	case <-exited:
		t.Fatalf("web stop signalled PID %d, which is `sleep`, not a peasant web server", cmd.Process.Pid)
	case <-time.After(300 * time.Millisecond):
	}
	if _, statErr := os.Stat(pidFile); !os.IsNotExist(statErr) {
		t.Errorf("stale PID file %s was kept (stat err %v)", pidFile, statErr)
	}
}

// TestStopWebStopsTheServerItStarted keeps the fallback working for a real
// match: a child whose command line is the detached server's
// (`… web start --foreground --port N`) is stopped.
func TestStopWebStopsTheServerItStarted(t *testing.T) {
	if os.Getenv(webServerStandInEnv) == "1" {
		time.Sleep(90 * time.Second)
		os.Exit(0)
	}
	port := freePort(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestStopWebStopsTheServerItStarted$", "--", "web", "start", "--foreground", "--port", strconv.Itoa(port))
	cmd.Env = append(os.Environ(), webServerStandInEnv+"=1")
	exited := startChild(t, cmd)
	writePIDFile(t, port, cmd.Process.Pid)
	// Give the child time to exec so its command line is the stand-in's.
	time.Sleep(200 * time.Millisecond)

	if err := stopWeb(port); err != nil {
		t.Fatalf("stopWeb(%d) on the server's own PID: %v", port, err)
	}
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatalf("server stand-in PID %d still running after web stop", cmd.Process.Pid)
	}
}
