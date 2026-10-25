package ingest

import (
	"fmt"
	"sort"
	"strings"
)

// duplicateCheckedHarness reports whether a harness stores one session per
// source file, so two files claiming one id are two different transcripts.
// OpenCode and Cursor can legitimately discover one session from two stores,
// so they are not checked.
func duplicateCheckedHarness(h Harness) bool {
	return h == HarnessClaudeCode || h == HarnessCodex
}

// refuseDuplicateSessionIDs removes every Claude or Codex session whose id more
// than one source file claims, and returns a failed result for each such id.
// Processing both files stored one row mixed from the two transcripts and
// raced their index writes while the run reported no errors. Neither file can
// be preferred safely, so the id fails until one file is moved aside. A
// session the selection rejects is left for the selection to skip.
func refuseDuplicateSessionIDs(fsys FileSystem, sessions []DiscoveredSession, selected func(DiscoveredSession) bool) ([]DiscoveredSession, []SessionResult) {
	type key struct {
		harness Harness
		id      SessionID
	}
	paths := make(map[key]map[ResolvedPath]DiscoveredSession)
	for _, s := range sessions {
		if !duplicateCheckedHarness(s.Harness) || (selected != nil && s.ParentUUID == nil && !selected(s)) {
			continue
		}
		k := key{s.Harness, s.SessionID}
		if paths[k] == nil {
			paths[k] = map[ResolvedPath]DiscoveredSession{}
		}
		paths[k][s.SourcePath] = s
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
		refused = append(refused, SessionResult{
			SessionID:  s.SessionID,
			Harness:    s.Harness,
			ParentUUID: s.ParentUUID,
			Status:     DiffUnchanged,
			Error:      duplicateSessionError(fsys, paths[k]),
		})
	}
	return kept, refused
}

// duplicateSessionError names every file, the newer and the larger one, and
// says to move one aside: either file may be the only copy of its transcript.
func duplicateSessionError(fsys FileSystem, claims map[ResolvedPath]DiscoveredSession) error {
	files := make([]string, 0, len(claims))
	for p := range claims {
		files = append(files, string(p))
	}
	sort.Strings(files)
	var newest, largest string
	var newestSession DiscoveredSession
	var largestSize int64 = -1
	for _, f := range files {
		s := claims[ResolvedPath(f)]
		if newest == "" || s.ModTime.After(newestSession.ModTime) {
			newest, newestSession = f, s
		}
		if fsys != nil {
			if info, err := fsys.Stat(f); err == nil && info.Size() > largestSize {
				largest, largestSize = f, info.Size()
			}
		}
	}
	which := "newer: " + newest
	if largest != "" {
		which += "; larger: " + largest
	}
	return fmt.Errorf("same session id in %d source files (%s; %s); none was ingested this run, so no stored row mixes them; "+
		"move the one you do not want out of the harness directory (do not delete it: it may be the only copy of that transcript) and rerun harvest",
		len(files), strings.Join(files, ", "), which)
}
