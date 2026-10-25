package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// errProcessProbe means the platform probe could not tell what a PID is (ps
// failed, the process table could not be read). `web stop` then keeps the PID
// file: deleting it could orphan a server that is really running.
var errProcessProbe = errors.New("cannot inspect the process")

// isWebServerCommandLine reports whether args is the command line `web start`
// gives the detached server it spawns (`<exe> web start --foreground --port N
// …`, see webServerSpawn.args) for this port. The executable itself is checked
// separately (sameExecutable), so an unrelated program started with the same
// trailing arguments is not a match.
func isWebServerCommandLine(args []string, port int) bool {
	want := strconv.Itoa(port)
	sawCommand, sawPort := false, false
	for i := range args {
		if i+2 < len(args) && args[i] == "web" && args[i+1] == "start" && args[i+2] == "--foreground" {
			sawCommand = true
		}
		if args[i] == "--port" && i+1 < len(args) && args[i+1] == want {
			sawPort = true
		}
		if strings.TrimPrefix(args[i], "--port=") == want && strings.HasPrefix(args[i], "--port=") {
			sawPort = true
		}
	}
	return sawCommand && sawPort
}

// sameExecutable reports whether path names the executable this process runs.
// An upgrade can replace the file under a running server, so Linux's
// " (deleted)" suffix is ignored and, when a full path cannot be compared, the
// base names must match.
func sameExecutable(path string) bool {
	self, err := os.Executable()
	if err != nil || path == "" {
		return false
	}
	path = strings.TrimSuffix(path, " (deleted)")
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	if filepath.IsAbs(path) {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		if path == self {
			return true
		}
	}
	return strings.EqualFold(filepath.Base(path), filepath.Base(self))
}
