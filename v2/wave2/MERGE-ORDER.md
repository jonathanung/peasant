# Merge order and conflict resolution (CHANGELOG wave), checked 2026-10-26 ~00:50 PDT

Base: origin/develop 81a173f8. Every PR below is 7 commits behind develop and merges into develop
**without conflict on its own**, so no PR was rebased. I did not rebase any PR onto another PR.
The throwaway sequential merge was in /tmp/wt-mergesim and was never pushed.

## Order
1. #704 3c6be354
2. #705 4e698da2
3. #707 a5745834
4. #708 aa7a6828
5. #711 6d9c29ce
6. #715 c42c2827
7. #732 37f25cb7 (docs). Keep it before #729, as stated earlier.
8. #729 060c2184
9. #752 769185e8. It goes last because it needs the #705 call-shape change below.

## Conflicts (pairwise, each PR on develop vs another; `git merge-tree`)
CHANGELOG.md is the only file that conflicts among 704/705/707/708/711/715/729/752. In every pair the
conflict is **one hunk**: both PRs insert entries at the top of `## [Unreleased]`.
- 704 vs 705, 708, 715, 729, 752. 705 vs 708, 715, 729, 752. 708 vs 715, 729, 752. 715 vs 729, 752. 729 vs 752.
  (This list is from the pre-wave heads. #707 and #711 had no CHANGELOG entry then. Now that they have
  one, they conflict the same way with every PR above. In the sequential run, every merge after #704
  hit exactly one CHANGELOG hunk.)
- **Semantic (no textual conflict), #705 vs #752.** #705 changes `loadRunConfig(path, dryRun)` to
  `loadRunConfig(cmd, path, dryRun)`, but #752's `cmd_service.go` (`runServiceInstall`) still calls
  `loadRunConfig(configPath, true)`. After both merge, `go build ./cmd/peasant` fails with
  "not enough arguments in call to loadRunConfig".
- **#732 (docs) vs #729 and #752: README.md.** This conflict happens in either order.
  - In the commands table, #752 adds the `peasant service` row, and #732 renames the `ingest`/`ingest verify` rows to `harvest`.
  - In the harvest flags table, #729 edits the `--since` row, and #732 rewrites the `--dry-run`/`--force`/`--all`/`--source-*` rows.

## Exact resolution
- **CHANGELOG.md.** Keep both sides, with the earlier-merged PR's entries first. Then keep one heading per
  type in Keep a Changelog order (Added, Changed, Fixed), with no duplicate `### Fixed`. Each entry
  stays verbatim, in merge order. Script: `evidence/wave2/changelog_union.py CHANGELOG.md`
  (`python3 changelog_union.py CHANGELOG.md`, then `git add CHANGELOG.md`). After all nine merges, the
  Unreleased section is:
  - `### Added`: #752 (peasant service).
  - `### Changed`: #707 (harvest --json Unchanged / ExcludedBySelection).
  - `### Fixed`: #704 (isMeta title), #705 (--data-dir), #708 (exit code), #711 (one harvest at a
    time), #715 (--port), #729 (--since), #752 (`peasant redact`).
- **#752 after #705.** In `cmd/peasant/cmd_service.go` `runServiceInstall`, change
  `cfg, err := loadRunConfig(configPath, true)` to `cfg, err := loadRunConfig(cmd, configPath, true)`.
  The check that follows (`checkIngestRedactionLevel`) does not read `Output.BasePath`, so applying
  `--data-dir` there has no behaviour change. It keeps every run loader on one shape.
- **README.md (#732 with #729/#752).**
  - Take #732's text for the rows it rewrites (`harvest` naming, the flag descriptions, `pi` in `--source-harness`).
  - Keep #752's `peasant service` row in the commands table.
  - Keep #729's `--since` row text.

## Verified on the throwaway merge of all nine (/tmp/wt-mergesim)
- With the CHANGELOG union and the one-line `cmd_service.go` fix: `go build ./...` ok, and
  `go vet ./cmd/peasant` ok.
- `go test -p 2 ./cmd/peasant -run 'Service|DataDir|RuntimeConfig|Concurrent|Selection|Excluded'`: ok (go1.25.8).
- The full suite was not run on the combined tree.
- The README hunks for #732 were inspected but not resolved in the run (#732 was aborted there).

## #723 vs #731: cmd/peasant/cmd_harvest.go (recorded 2026-10-26 PDT)
Heads: #723 bfe14206 (contains #721, #722), #731 f30cfb1c, develop 81a173f8. Conflict appears when #731 merges onto #723 (any order: same hunk).
- Textual: #723 moved the verify header/schema/table-presence block from runVerify into `verifyTablePresence(out, db, dbPath, expectedTables, moreChecks)`; #731 changed the open in runVerify to `openReadOnlyIfUnwritable(cmd)` with a `store.Open` fallback and an early `if content { return runVerifyContent(...) }`.
- Semantic (found by the test, not by git): #723 reads the version with `store.SchemaVersionAt(dbPath)`, a fresh plain read-only open, which fails on a read-only data dir (`attempt to write a readonly database`) so TestReadOnlyDataDirServesReadOnlyCommands fails for `harvest verify` and `verify --content` with only the textual fix.

Resolution (verified):
1. Take #723's side of the hunk (keep `verifyTablePresence` and #723's runVerify flow, including its own --content hand-off after the presence check).
2. In #723's runVerify replace `db, err := store.Open(dbPath)` with #731's open:
```go
	// verify only reads: on a read-only data dir open the database read-only.
	db, err := openReadOnlyIfUnwritable(cmd)
	if err == nil && db == nil {
		db, err = store.Open(dbPath)
	}
```
3. In verifyTablePresence read the version through the open handle: `foundVersion, err := store.SchemaVersionFromDB(db)`, adding to internal/store/store.go:
```go
func SchemaVersionFromDB(s *Store) (int, error) {
	return readUserVersion(s.pool)
}
```
Full resolution diff vs #723 head:
```diff
diff --git a/cmd/peasant/cmd_harvest.go b/cmd/peasant/cmd_harvest.go
index 4a98bb24..0babdcaa 100644
--- a/cmd/peasant/cmd_harvest.go
+++ b/cmd/peasant/cmd_harvest.go
@@ -883,7 +883,9 @@ func verifyTablePresence(out io.Writer, db *store.Store, dbPath string, expected
 		return fmt.Errorf("get table names: %w", err)
 	}
 
-	foundVersion, err := store.SchemaVersionAt(dbPath)
+	// Read the version through the open handle: on a read-only data dir it is
+	// the immutable read-only store, and a fresh plain open would fail.
+	foundVersion, err := store.SchemaVersionFromDB(db)
 	if err != nil {
 		return err
 	}
@@ -937,7 +939,11 @@ func runVerify(cmd *cobra.Command, verbose, content, repair bool) error {
 	ctx := cmd.Context()
 	dbPath := string(defaults.ResolveDBFilePathWith(dataDirOverride(cmd)))
 
-	db, err := store.Open(dbPath)
+	// verify only reads: on a read-only data dir open the database read-only.
+	db, err := openReadOnlyIfUnwritable(cmd)
+	if err == nil && db == nil {
+		db, err = store.Open(dbPath)
+	}
 	if err != nil {
 		return fmt.Errorf("open database: %w", err)
 	}
diff --git a/internal/store/store.go b/internal/store/store.go
index 4f319e07..cad847ba 100644
--- a/internal/store/store.go
+++ b/internal/store/store.go
@@ -51,6 +51,13 @@ func resolvePoolSize(optPoolSize int) int {
 }
 
 // AllTableNamesFromDB retrieves the list of tables from the database.
+// SchemaVersionFromDB returns PRAGMA user_version read through an open store,
+// so it works on any store the caller could open, including an immutable
+// read-only one.
+func SchemaVersionFromDB(s *Store) (int, error) {
+	return readUserVersion(s.pool)
+}
+
 func AllTableNamesFromDB(s *Store) ([]string, error) {
 	conn, err := s.pool.Take(context.Background())
 	if err != nil {
@@ -321,6 +328,23 @@ type openOptions struct {
 	// count; the pipeline passes its resolved configuration so the store
 	// lanes ask the same splitter over the same knobs.
 	writeConfig *ingest.WriteConfig
+	// immutable opens a read-only store with SQLite's immutable=1 so it
+	// needs no -shm file; see WithImmutableReadOnly.
+	immutable bool
+}
+
+// WithImmutableReadOnly makes OpenReadOnlyWithOptions open the database with
+// SQLite's immutable=1, for a data directory the process cannot write. A WAL
+// database otherwise needs to create its -shm file next to the database even
+// to read, and fails with "attempt to write a readonly database". The open is
+// refused while a non-empty -wal file exists, because immutable reads would
+// skip the committed frames in it.
+// immutable=1 also tells SQLite no other process writes the file while it is
+// open; on a directory this process cannot write that is normally true, but a
+// writer with more permissions changing the file meanwhile would not be seen
+// and could make reads inconsistent.
+func WithImmutableReadOnly() OpenOption {
+	return func(o *openOptions) { o.immutable = true }
 }
 
 // WithWriteConfig gives the store's staging and activation lanes their
```
Verified in a throwaway local merge (bfe14206 + f30cfb1c, never pushed, removed): gofmt clean, go vet ./cmd/peasant ./internal/store ok, go1.25.8 `go test -count=1 -run 'TestReadOnlyDataDir|TestImmutableReadOnly|Verify' ./cmd/peasant ./internal/store` ok. Whichever of #723/#731 merges second needs this rebase (step 3 is a code addition the second PR must carry).
