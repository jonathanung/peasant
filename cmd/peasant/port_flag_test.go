package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestPortFlagsRejectOutOfRangePorts guards #681: --port outside 1-65535 is
// refused while the flag is parsed, so cobra stops before RunE: no server is
// spawned and no web-<port>.pid is written. Parsing the production commands'
// own flag sets keeps the test from starting a server on develop.
func TestPortFlagsRejectOutOfRangePorts(t *testing.T) {
	t.Parallel()
	web := BuildWebCommand()
	find := func(root *cobra.Command, name string) *cobra.Command {
		t.Helper()
		sub, _, err := root.Find([]string{name})
		if err != nil || sub == root {
			t.Fatalf("find %s: %v", name, err)
		}
		return sub
	}
	commands := map[string]*cobra.Command{
		"web start": find(web, "start"),
		"web stop":  find(web, "stop"),
		"open":      BuildOpenCommand(),
	}
	for name, cmd := range commands {
		for _, port := range []string{"0", "-1", "65536", "70000"} {
			err := cmd.ParseFlags([]string{"--port", port})
			if err == nil || !strings.Contains(err.Error(), "1-65535") {
				t.Errorf("%s --port %s: parse err = %v, want a range error naming 1-65535", name, port, err)
			}
		}
		for _, port := range []string{"1", "8690", "65535"} {
			if err := cmd.ParseFlags([]string{"--port", port}); err != nil {
				t.Errorf("%s --port %s: parse err = %v, want it accepted", name, port, err)
			}
		}
	}
}
