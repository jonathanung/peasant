//go:build unix && !linux

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// isPeasantWebServer asks ps for the process's executable and command line
// (macOS and the BSDs have no /proc). -ww stops ps truncating the line to the
// terminal width. ps exits 1 with no output when the PID does not exist; that
// is not a match. Any other failure is a probe error.
func isPeasantWebServer(pid, port int) (bool, error) {
	out, err := exec.Command("ps", "-ww", "-o", "comm=", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(strings.TrimSpace(string(out))) == 0 {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w %d: %v", errProcessProbe, pid, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return false, nil
	}
	// comm is the executable path; args repeats argv[0] and the arguments.
	// The server's own arguments contain no spaces.
	return sameExecutable(fields[0]) && isWebServerCommandLine(fields[1:], port), nil
}
