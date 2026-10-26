package kickstart_test

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/peasant-labs/peasant/internal/tui/ftue"
	"github.com/peasant-labs/peasant/internal/tui/kickstart"
)

// TestProgramDoneShowsSessionsExcludedBySelection: the local import result
// no longer counts sessions the selection left out as unchanged, so the
// mounted kickstart completion must name them, or they vanish from the
// screen. A run with none excluded keeps the old line.
func TestProgramDoneShowsSessionsExcludedBySelection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		excluded int
		want     string
		absent   string
	}{
		{excluded: 4, want: "4 excluded by selection"},
		{excluded: 0, absent: "excluded by selection"},
	} {
		result := &ftue.IngestResult{New: 2, Updated: 1, Unchanged: 3, ExcludedBySelection: row.excluded}
		program, _ := newTestProgram(t, kickstart.ProgramDeps{AlreadyConnected: true, Ingest: func(context.Context) (*ftue.IngestResult, error) { return result, nil }})
		program.SetSize(120, 40)
		program = drainProgram(program, program.Init())
		program, command := advanceToCommit(program)
		program = drainCmds(t, program, command)
		if program.Phase() != kickstart.PhaseDone {
			t.Fatalf("excluded=%d: phase = %s, want done", row.excluded, program.Phase())
		}
		view := strings.Join(strings.Fields(ansi.Strip(program.View())), " ")
		if row.want != "" && !strings.Contains(view, row.want) {
			t.Errorf("excluded=%d: completion omits %q:\n%s", row.excluded, row.want, view)
		}
		if row.absent != "" && strings.Contains(view, row.absent) {
			t.Errorf("excluded=%d: completion mentions %q:\n%s", row.excluded, row.absent, view)
		}
	}
}
