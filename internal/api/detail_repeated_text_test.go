package api

import (
	"testing"

	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/schema"
)

func repeatedTextTurns(childTexts ...string) []schema.TurnDetail {
	parent := 0
	turns := []schema.TurnDetail{{Index: 0, Depth: 0, Role: "assistant", EntryType: schema.EntryTypeText, Content: "ok"}}
	for i, text := range childTexts {
		turns = append(turns, schema.TurnDetail{Index: i + 1, Depth: 1, ParentIndex: &parent, Role: "assistant", EntryType: schema.EntryTypeText, Content: text})
	}
	return turns
}

// TestFoldRepeatedTextOnlyFoldsTheSingleClaudeBlock: the fold removes only a
// Claude message's one text block that repeats it. Two identical blocks are
// real repeats and stay, as does any other harness's detail.
func TestFoldRepeatedTextOnlyFoldsTheSingleClaudeBlock(t *testing.T) {
	t.Parallel()
	claude := string(defaults.HarnessClaudeCode)
	for _, tc := range []struct {
		name    string
		harness string
		turns   []schema.TurnDetail
		want    int
	}{
		{"single repeated Claude block", claude, repeatedTextTurns("ok"), 1},
		{"two identical Claude blocks", claude, repeatedTextTurns("ok", "ok"), 3},
		{"single block with other text", claude, repeatedTextTurns("different"), 2},
		{"single repeated block, other harness", string(defaults.HarnessCodex), repeatedTextTurns("ok"), 2},
	} {
		if got := len(foldRepeatedTextTurns(tc.harness, tc.turns)); got != tc.want {
			t.Errorf("%s: %d turns, want %d", tc.name, got, tc.want)
		}
	}
}
