package api

import (
	"github.com/peasant-labs/peasant/internal/defaults"
	"github.com/peasant-labs/schema"
)

// foldRepeatedTextTurns drops the depth-1 text turn of a Claude message whose
// only child is that one text block repeating the message. A single-block
// Claude assistant message is stored as the message and its one text block
// with the same text, and both reached the local detail read, so every reply
// showed twice, the copy with a zero timestamp. A message with two or more
// children keeps all of them, identical or not, because those are real
// blocks, and other harnesses do not store that shape. This is the local read
// only: the durable detail that export and publication serialize keeps the
// block, so their bytes and content hash do not move.
func foldRepeatedTextTurns(harness string, turns []schema.TurnDetail) []schema.TurnDetail {
	if harness != string(defaults.HarnessClaudeCode) {
		return turns
	}
	parents := make(map[int]*schema.TurnDetail, len(turns))
	children := make(map[int]int, len(turns))
	for i := range turns {
		switch {
		case turns[i].Depth == 0:
			parents[turns[i].Index] = &turns[i]
		case turns[i].Depth == 1 && turns[i].ParentIndex != nil:
			children[*turns[i].ParentIndex]++
		}
	}
	kept := turns[:0:0]
	for _, turn := range turns {
		if turn.Depth == 1 && turn.ParentIndex != nil && children[*turn.ParentIndex] == 1 &&
			turn.EntryType == schema.EntryTypeText && len(turn.ToolCalls) == 0 {
			if parent, ok := parents[*turn.ParentIndex]; ok && parent.Role == turn.Role && parent.Content == turn.Content {
				continue
			}
		}
		kept = append(kept, turn)
	}
	return kept
}
