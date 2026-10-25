//go:build unix && !linux

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// isPeasantWebServer asks ps for the process's command line (macOS and the
// BSDs have no /proc). ps splits nothing for us, so the line is split on
// spaces; the server's own arguments contain none. A process ps cannot report
// is not a match.
func isPeasantWebServer(pid, port int) bool {
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	return isWebServerCommandLine(strings.Fields(string(out)), port)
}
