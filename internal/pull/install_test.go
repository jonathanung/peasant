package pull_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peasant-labs/peasant/internal/ingest"
	"github.com/peasant-labs/peasant/internal/pull"
	"github.com/peasant-labs/peasant/internal/store"
	"github.com/peasant-labs/peasant/internal/store/storetest"
	"github.com/peasant-labs/peasant/internal/testutil"
	"github.com/peasant-labs/peasant/internal/village"
	"github.com/peasant-labs/schema"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/content-install.yaml
var contentInstallYAML []byte

type installCase struct {
	Name            string `yaml:"name"`
	Operation       string `yaml:"operation"`
	Target          string `yaml:"target"`
	After           bool   `yaml:"after"`
	ForceRecovery   bool   `yaml:"forceRecovery"`
	Mode            string `yaml:"mode"`
	CorruptPrevious bool   `yaml:"corruptPrevious"`
}

// A second process must not sweep or replace the first process's staging tree.
// This bounded subprocess is necessary because goroutines cannot detect a
// mistaken in-process-only mutex. It uses the actual production pull entry,
// filesystem, SQLite store and advisory-lock implementation.
func TestContentInstallProcessLock(t *testing.T) {
	const rootEnv = "PEASANT_PULL_LOCK_TEST_ROOT"
	const heldEnv = "PEASANT_PULL_LOCK_TEST_HELD"
	if root := os.Getenv(rootEnv); root != "" {
		db, err := store.Open(filepath.Join(root, "index.db"), store.WithSkipMigrations())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		deadline := 5 * time.Second
		if os.Getenv(heldEnv) == "true" {
			deadline = 150 * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		_, err = pull.NewPipeline(happyReader(), &ingest.OSFileSystem{}, db, testutil.NewFixedClock(), testCreds(), filepath.Join(root, "pulls")).PullTranscript(ctx, newRef(t), pull.PullOptions{Force: true})
		if os.Getenv(heldEnv) == "true" {
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("concurrent process did not honor content ownership: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		return
	}
	root := t.TempDir()
	storetest.CopyGoldenTo(t, filepath.Join(root, "index.db"))
	db, err := store.Open(filepath.Join(root, "index.db"), store.WithSkipMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	pulls := filepath.Join(root, "pulls")
	hostRoot := filepath.Join(pulls, testutil.TestVillageHost)
	locker, err := store.NewFileSessionLocker(filepath.Join(hostRoot, ".pull-locks"))
	if err != nil {
		t.Fatal(err)
	}
	lockID, err := schema.NewSessionID(testutil.TestTranscriptUUID)
	if err != nil {
		t.Fatal(err)
	}
	release, err := locker.LockExclusive(context.Background(), lockID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if release != nil {
			_ = release()
		}
	})
	owned := filepath.Join(hostRoot, ".install-"+testutil.TestTranscriptUUID)
	if err := os.MkdirAll(owned, 0o700); err != nil {
		t.Fatal(err)
	}
	unowned := filepath.Join(owned, testutil.TestOwnerUserID)
	if err := os.MkdirAll(unowned, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(unowned, "unrelated-note")
	if err := os.WriteFile(sentinel, []byte("retain"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(held string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestContentInstallProcessLock$", "-test.count=1")
		command.Env = append(os.Environ(), rootEnv+"="+root, heldEnv+"="+held)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("child production pull: %v\n%s", err, output)
		}
	}
	run("true")
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "retain" {
		t.Fatalf("waiting process changed owned resources: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	release = nil
	run("false")
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "retain" {
		t.Fatalf("recovery deleted an unowned file: %v", err)
	}
}

func TestContentInstallDryRunDoesNotRepairOrWrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storetest.CopyGoldenTo(t, filepath.Join(root, "index.db"))
	db, err := store.Open(filepath.Join(root, "index.db"), store.WithSkipMigrations())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pulls := filepath.Join(root, "pulls")
	osfs := &ingest.OSFileSystem{}
	fault := &installFaultStore{PullStore: db}
	if _, err := pull.NewPipeline(happyReader(), osfs, fault, testutil.NewFixedClock(), testCreds(), pulls).PullTranscript(ctx, newRef(t), pull.PullOptions{}); !errors.Is(err, errInstallInjected) {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(pulls, testutil.TestVillageHost, ".install-"+testutil.TestTranscriptUUID, "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	filesystem := testutil.NewFailingFS(osfs)
	filesystem.WriteFileErr = errors.New("dry run wrote files")
	filesystem.RenameErr = errors.New("dry run published files")
	filesystem.RemoveAllErr = errors.New("dry run cleaned recovery resources")
	filesystem.RemoveErr = errors.New("dry run removed recovery journal")
	neverCommit := &installFaultStore{PullStore: db}
	reader := happyReader()
	reader.Meta.Title = "Must not install"
	result, err := pull.NewPipeline(reader, filesystem, neverCommit, testutil.NewFixedClock(), testCreds(), pulls).PullTranscript(ctx, newRef(t), pull.PullOptions{DryRun: true})
	if err != nil || !result.DryRun ||
		neverCommit.fired || reader.ContentCalls != 0 ||
		reader.AnnotationsCalls != 0 {
		t.Fatalf("dry run performed mutation/transfer: %+v %v", result, err)
	}
	after, err := os.ReadFile(filepath.Join(pulls, testutil.TestVillageHost, ".install-"+testutil.TestTranscriptUUID, "journal.json"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("dry run changed pending recovery: %v", err)
	}
}

func loadContentInstallFixtures(t *testing.T) []installCase {
	t.Helper()
	var corpus struct {
		RequiredNames []string      `yaml:"requiredNames"`
		Cases         []installCase `yaml:"cases"`
	}
	if err := yaml.Unmarshal(contentInstallYAML, &corpus); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, c := range corpus.Cases {
		if c.Name == "" || seen[c.Name] {
			t.Fatalf("empty/duplicate fixture name: %q", c.Name)
		}
		seen[c.Name] = true
	}
	for _, name := range corpus.RequiredNames {
		if !seen[name] {
			t.Fatalf("required content-install fixture missing: %s", name)
		}
	}
	return corpus.Cases
}

var errInstallInjected = errors.New("injected filesystem/index interruption")

// This decorator faults actual OS operations, never the pipeline under test.
type installFaultFS struct {
	ingest.FileSystem
	fixture installCase
	final   string
	fired   bool
}

var _ ingest.FileSystem = (*installFaultFS)(nil)

func (f *installFaultFS) operation(op, path string, run func() error) error {
	match := false
	switch f.fixture.Target {
	case "metadata":
		match = filepath.Base(path) == pull.MetadataFilename
	case "journal":
		match = filepath.Base(path) == "journal.json"
	case "backup":
		match = filepath.Base(path) == "previous"
	case "publish":
		match = path == f.final
	case "metadataPublish":
		match = path == filepath.Join(f.final, pull.MetadataFilename)
	case "metadataBackup":
		match = filepath.Base(path) == "previous-metadata.json"
	case "metadataBackupTemp":
		match = filepath.Base(path) == "previous-metadata.json.tmp"
	}
	if f.fired || op != f.fixture.Operation || !match {
		return run()
	}
	f.fired = true
	if f.fixture.After {
		if err := run(); err != nil {
			return err
		}
	}
	return errInstallInjected
}
func (f *installFaultFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	return f.operation("write", path, func() error { return f.FileSystem.WriteFile(path, data, mode) })
}
func (f *installFaultFS) Rename(src, dst string) error {
	return f.operation("rename", dst, func() error { return f.FileSystem.Rename(src, dst) })
}
func (f *installFaultFS) RemoveAll(path string) error {
	return f.operation("removeAll", path, func() error { return f.FileSystem.RemoveAll(path) })
}
func (f *installFaultFS) Remove(path string) error {
	return f.operation("remove", path, func() error { return f.FileSystem.Remove(path) })
}

func (f *installFaultFS) CopyFile(src, dst string, mode os.FileMode) error {
	return f.operation("copy", dst, func() error { return f.FileSystem.CopyFile(src, dst, mode) })
}

type installFaultStore struct {
	pull.PullStore
	after bool
	fired bool
}

var _ pull.PullStore = (*installFaultStore)(nil)

func (s *installFaultStore) CommitPull(ctx context.Context, commit store.PullCommit) error {
	if s.fired {
		return s.PullStore.CommitPull(ctx, commit)
	}
	s.fired = true
	if s.after {
		if err := s.PullStore.CommitPull(ctx, commit); err != nil {
			return err
		}
	}
	return errInstallInjected
}

func TestContentInstallRestartRecovery(t *testing.T) {
	for _, fixture := range loadContentInstallFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			root := t.TempDir()
			dbPath := filepath.Join(root, "index.db")
			storetest.CopyGoldenTo(t, dbPath)
			db, err := store.Open(dbPath, store.WithSkipMigrations())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			pulls := filepath.Join(root, "pulls")
			osfs := &ingest.OSFileSystem{}
			ref := newRef(t)
			first := happyReader()
			if _, err := pull.NewPipeline(first, osfs, db, testutil.NewFixedClock(), testCreds(), pulls).PullTranscript(ctx, ref, pull.PullOptions{}); err != nil {
				t.Fatal(err)
			}
			final := filepath.Join(pulls, testutil.TestVillageHost, ref.ID.String())
			oldBlob, err := os.ReadFile(filepath.Join(final, pull.TranscriptFilename))
			if err != nil {
				t.Fatal(err)
			}
			oldBlobInfo, err := os.Stat(filepath.Join(final, pull.TranscriptFilename))
			if err != nil {
				t.Fatal(err)
			}
			oldMetadata, err := os.ReadFile(filepath.Join(final, pull.MetadataFilename))
			if err != nil {
				t.Fatal(err)
			}
			oldManifest, err := os.ReadFile(filepath.Join(final, pull.ManifestFilename))
			if err != nil {
				t.Fatal(err)
			}
			if fixture.CorruptPrevious {
				if err := os.Remove(filepath.Join(final, pull.MetadataFilename)); err != nil {
					t.Fatal(err)
				}
			}
			reader := happyReader()
			reader.Meta.Title = "Updated metadata"
			force := fixture.Mode == ""
			if force {
				reader.ContentBody = []byte(`{"updated":true}`)
				reader.ContentETag = quotedETag(schema.ComputeTranscriptHash(reader.ContentBody))
				reader.Meta.ContentHash = schema.ComputeTranscriptHash(reader.ContentBody)
			}
			switch fixture.Mode {
			case "etag":
				reader.Meta.ContentHash = ""
				reader.ContentErr = village.ErrNotModified
			case "download":
				reader.Meta.ContentHash = ""
			}
			filesystem := &installFaultFS{FileSystem: osfs, fixture: fixture, final: final}
			var persistence pull.PullStore = db
			var failedStore *installFaultStore
			if fixture.Operation == "index" {
				failedStore = &installFaultStore{PullStore: db, after: fixture.After}
				persistence = failedStore
			}
			result, err := pull.NewPipeline(reader, filesystem, persistence, testutil.NewFixedClock(), testCreds(), pulls).PullTranscript(ctx, ref, pull.PullOptions{Force: force})
			if !errors.Is(err, errInstallInjected) || result.Status != pull.PullStatusError {
				t.Fatalf("fault must be a required failure, result=%+v error=%v", result, err)
			}
			if !filesystem.fired && (failedStore == nil || !failedStore.fired) {
				t.Fatal("fault did not hit actual production operation")
			}
			// The previous complete files must survive in the published directory
			// or the owned backup until the replacement index is verified.
			backup := filepath.Join(pulls, testutil.TestVillageHost, ".install-"+ref.ID.String(), "previous")
			oldDir := final
			if _, err := os.Stat(backup); err == nil {
				oldDir = backup
			}
			if fixture.Target != "journal" || fixture.Operation != "remove" {
				got, err := os.ReadFile(filepath.Join(oldDir, pull.TranscriptFilename))
				if err != nil || string(got) != string(oldBlob) {
					t.Fatalf("previous complete blob lost before verified projection: %q %v", got, err)
				}
			}
			if fixture.Mode != "" {
				currentBlobInfo, err := os.Stat(filepath.Join(final, pull.TranscriptFilename))
				if err != nil || !os.SameFile(oldBlobInfo, currentBlobInfo) {
					t.Fatalf("metadata-only update moved or replaced content: %v", err)
				}
				currentMetadata, err := os.ReadFile(filepath.Join(final, pull.MetadataFilename))
				if err != nil {
					t.Fatal(err)
				}
				if string(currentMetadata) != string(oldMetadata) && fixture.Operation != "remove" {
					previous, err := os.ReadFile(filepath.Join(filepath.Dir(backup), "previous-metadata.json"))
					if err != nil || string(previous) != string(oldMetadata) {
						t.Fatalf("previous metadata lost before verified projection: %v", err)
					}
				}
				got, err := os.ReadFile(filepath.Join(final, pull.ManifestFilename))
				if err != nil || string(got) != string(oldManifest) {
					t.Fatalf("metadata-only update changed provenance: %v", err)
				}
				if (fixture.Mode == "hash" && reader.ContentCalls != 0) ||
					(fixture.Mode == "etag" && reader.ContentCalls != 1) {
					t.Fatal("metadata-only update transferred blob")
				}
			}
			// Reopen the actual SQLite store and construct a fresh pipeline: no
			// in-memory journal state or manual SQL is available during recovery.
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = store.Open(dbPath, store.WithSkipMigrations())
			if err != nil {
				t.Fatal(err)
			}
			retry := happyReader()
			retry.Meta = reader.Meta
			retry.ContentBody = reader.ContentBody
			retry.ContentETag = reader.ContentETag
			result, err = pull.NewPipeline(retry, osfs, db, testutil.NewFixedClock(), testCreds(), pulls).PullTranscript(ctx, ref, pull.PullOptions{Force: fixture.ForceRecovery})
			if err != nil || (result.Status != pull.PullStatusPulled && result.Status != pull.PullStatusUpToDate) {
				t.Fatalf("restart repair: result=%+v error=%v", result, err)
			}
			row, err := db.GetPulledTranscript(ctx, testutil.TestVillageHost, ref.ID)
			if err != nil || row == nil || row.Title != reader.Meta.Title {
				t.Fatalf("index projection was not repaired: row=%+v error=%v", row, err)
			}
			var installed schema.PullTranscriptInfo
			data, err := os.ReadFile(filepath.Join(final, pull.MetadataFilename))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &installed); err != nil || installed.Title != row.Title {
				t.Fatalf("files/index disagree: %+v %v", installed, err)
			}
			entries, err := os.ReadDir(filepath.Dir(backup))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".tmp") {
					t.Errorf("owned recovery resource leaked after repair: %s", entry.Name())
				}
			}
		})
	}
}
