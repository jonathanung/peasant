package main

import "testing"

// TestIsWebServerCommandLine pins which command lines `web stop` accepts as
// the detached server for a port, on every platform that reads one.
func TestIsWebServerCommandLine(t *testing.T) {
	t.Parallel()
	spawned := webServerSpawn{port: 18693, configPath: "/cfg/config.yaml"}.args()
	for name, tc := range map[string]struct {
		args []string
		want bool
	}{
		"the args web start spawns": {append([]string{"/usr/local/bin/peasant"}, spawned...), true},
		"--port=N form":             {[]string{"peasant", "web", "start", "--foreground", "--port=18693"}, true},
		"another port":              {[]string{"peasant", "web", "start", "--foreground", "--port", "18694"}, false},
		"not the foreground server": {[]string{"peasant", "web", "start", "--port", "18693"}, false},
		"an unrelated process":      {[]string{"sleep", "300"}, false},
		"port only, wrong command":  {[]string{"python3", "-m", "http.server", "--port", "18693"}, false},
		"empty (process is gone)":   {nil, false},
	} {
		if got := isWebServerCommandLine(tc.args, 18693); got != tc.want {
			t.Errorf("%s: isWebServerCommandLine(%q) = %v, want %v", name, tc.args, got, tc.want)
		}
	}
}
