package main

import (
	"strconv"
	"strings"
)

// isWebServerCommandLine reports whether args is the command line `web start`
// gives the detached server it spawns (`<exe> web start --foreground --port N
// …`, see webServerSpawn.args) for this port. `web stop` signals a PID only
// when this holds, so a stale PID file naming an unrelated process is never
// acted on.
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
