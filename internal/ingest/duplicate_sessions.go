package ingest

import (
	"fmt"
	"sort"
	"strings"
)

// refuseDuplicateSessionIDs removes every session whose id more than one source
// file of the same harness claims, and returns a failed result for each such
// id. Processing both files stored one row mixed from the two transcripts and
// raced their index writes while the run reported no errors. Neither file can
// be preferred safely, so the id fails until one of them is removed or renamed.
func refuseDuplicateSessionIDs(sessions []DiscoveredSession) ([]DiscoveredSession, []SessionResult) {
	type key struct {
		harness Harness
		id      SessionID
	}
	paths := make(map[key]map[ResolvedPath]bool)
	for _, s := range sessions {
		k := key{s.Harness, s.SessionID}
		if paths[k] == nil {
			paths[k] = map[ResolvedPath]bool{}
		}
		paths[k][s.SourcePath] = true
	}
	kept := sessions[:0:0]
	var refused []SessionResult
	reported := map[key]bool{}
	for _, s := range sessions {
		k := key{s.Harness, s.SessionID}
		if len(paths[k]) < 2 {
			kept = append(kept, s)
			continue
		}
		if reported[k] {
			continue
		}
		reported[k] = true
		files := make([]string, 0, len(paths[k]))
		for p := range paths[k] {
			files = append(files, string(p))
		}
		sort.Strings(files)
		refused = append(refused, SessionResult{
			SessionID:  s.SessionID,
			Harness:    s.Harness,
			ParentUUID: s.ParentUUID,
			Status:     DiffUnchanged,
			Error: fmt.Errorf("same session id in %d source files (%s); none was ingested this run, so no stored row mixes them; "+
				"remove or rename the duplicate file and rerun harvest", len(files), strings.Join(files, ", ")),
		})
	}
	return kept, refused
}
