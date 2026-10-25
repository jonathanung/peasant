//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
)

// isPeasantWebServer reads the process's command line from /proc. An
// unreadable entry (the process is gone, or belongs to someone else) is not a
// match.
func isPeasantWebServer(pid, port int) bool {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	var args []string
	for _, field := range bytes.Split(bytes.TrimRight(raw, "\x00"), []byte{0}) {
		args = append(args, string(field))
	}
	return isWebServerCommandLine(args, port)
}
