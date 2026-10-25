package api

import "github.com/peasant-labs/schema"

// foldRepeatedTextTurns drops a depth-1 text turn that only repeats its
// depth-0 parent. A single-block Claude assistant message is stored as the
// message and its one text block with the same text, and both reached the
// local detail read, so every reply showed twice, the copy with a zero
// timestamp. This is the local read only: the durable detail that export and
// publication serialize keeps the block, so their bytes and content hash do
// not move.
func foldRepeatedTextTurns(turns []schema.TurnDetail) []schema.TurnDetail {
	parents := make(map[int]*schema.TurnDetail, len(turns))
	for i := range turns {
		if turns[i].Depth == 0 {
			parents[turns[i].Index] = &turns[i]
		}
	}
	kept := turns[:0:0]
	for _, turn := range turns {
		if turn.Depth == 1 && turn.ParentIndex != nil && turn.EntryType == schema.EntryTypeText && len(turn.ToolCalls) == 0 {
			if parent, ok := parents[*turn.ParentIndex]; ok && parent.Role == turn.Role && parent.Content == turn.Content {
				continue
			}
		}
		kept = append(kept, turn)
	}
	return kept
}
