package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// noTerminalError reports that an interactive command could not reach a
// terminal. It replaces Bubble Tea's raw "could not open TTY" error so a piped,
// scripted, or detached invocation gets one actionable line and no usage dump.
type noTerminalError struct{ command string }

func (e noTerminalError) Error() string {
	return fmt.Sprintf("`peasant %s` needs an interactive terminal; run it from a terminal session (to ingest without the wizard, use `peasant harvest`)", e.command)
}

// terminalAvailable reports whether a Bubble Tea program can reach a terminal:
// either stdin is one, or the controlling terminal can be opened. It is a
// variable so tests can simulate a detached process.
var terminalAvailable = func() bool {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = tty.Close()
	return true
}

// requireTerminal is the check every production Bubble Tea boundary runs
// before it starts a program.
func requireTerminal(command string) error {
	if !terminalAvailable() {
		return noTerminalError{command: command}
	}
	return nil
}

// cleanTerminalError unwraps a noTerminalError from a command's error chain,
// returns it on its own, and silences cobra's usage block, which only helps
// with argument mistakes.
func cleanTerminalError(cmd *cobra.Command, err error) error {
	var noTTY noTerminalError
	if errors.As(err, &noTTY) {
		cmd.SilenceUsage = true
		return noTTY
	}
	return err
}

// withCleanTerminalError wraps cmd's RunE with cleanTerminalError.
func withCleanTerminalError(cmd *cobra.Command) *cobra.Command {
	inner := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return cleanTerminalError(c, inner(c, args))
	}
	return cmd
}
