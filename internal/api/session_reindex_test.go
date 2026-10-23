package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/peasant-labs/peasant/internal/config"
	"github.com/peasant-labs/peasant/internal/ingest"
	"github.com/peasant-labs/peasant/internal/store"
	"github.com/peasant-labs/peasant/internal/testutil"
	"github.com/peasant-labs/schema"
)

//go:embed testdata/session_reindex.yaml
var sessionReindexYAML []byte

type sessionReindexCase struct {
	Name                    string `yaml:"name"`
	Turns                   int    `yaml:"turns"`
	NeighbourTurns          int    `yaml:"neighbour_turns"`
	AppendTurns             int    `yaml:"append_turns"`
	AppendPartial           bool   `yaml:"append_partial"`
	AppendMalformed         bool   `yaml:"append_malformed"`
	NeighbourAppendTurns    int    `yaml:"neighbour_append_turns"`
	Unrecorded              bool   `yaml:"unrecorded"`
	Busy                    bool   `yaml:"busy"`
	Timeout                 string `yaml:"timeout"`
	Status                  int    `yaml:"status"`
	Code                    string `yaml:"code"`
	Message                 string `yaml:"message"`
	UserTurnsAfter          int    `yaml:"user_turns_after"`
	NeighbourUserTurnsAfter int    `yaml:"neighbour_user_turns_after"`
	ScanStatus              int    `yaml:"scan_status"`
}

type sessionReindexFixture struct {
	Required        []string             `yaml:"required_names"`
	Turn            string               `yaml:"turn"`
	SurrogateRecord string               `yaml:"surrogate_record"`
	PartialRecord   string               `yaml:"partial_record"`
	MalformedRecord string               `yaml:"malformed_record"`
	Cases           []sessionReindexCase `yaml:"cases"`
}

func loadSessionReindexFixture(t *testing.T) sessionReindexFixture {
	t.Helper()
	var fixture sessionReindexFixture
	if err := testutil.DecodeFixtureYAML(sessionReindexYAML, &fixture); err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool, len(fixture.Cases))
	for _, c := range fixture.Cases {
		if c.Name == "" || present[c.Name] {
			t.Fatalf("invalid session re-index fixture %q", c.Name)
		}
		present[c.Name] = true
	}
	if err := testutil.RequireFixtureNames("testdata/session_reindex.yaml", "session re-index", fixture.Required, present); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// turns renders turns from..to-1 of a session from the fixture's turn template.
func (f sessionReindexFixture) turns(sessionID string, from, to int) string {
	var out strings.Builder
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for turn := from; turn < to; turn++ {
		record := strings.ReplaceAll(f.Turn, "SESSION_ID", sessionID)
		record = strings.ReplaceAll(record, "TURN", strconv.Itoa(turn))
		out.WriteString(strings.ReplaceAll(record, "TIME", base.Add(time.Duration(turn)*time.Second).Format(time.RFC3339)))
	}
	return out.String()
}

func appendToFile(t *testing.T, path, data string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(data); err != nil {
		t.Fatal(err)
	}
}

// userTurns counts the stored top-level user messages of a session.
func userTurns(t *testing.T, db *store.Store, id string) int {
	t.Helper()
	sid, err := ingest.NewSessionID(id)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := db.ListEntries(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Depth == 0 && entry.Role == schema.RoleUser {
			count++
		}
	}
	return count
}

// TestSessionReindexRoute drives the mounted one-session re-index over a real
// Claude Code source and SQLite: the run re-reads the session only, from a
// stable view of a file still being written, answers within its bound, and
// names why it could not re-index; the scan then reads the fresh index.
func TestSessionReindexRoute(t *testing.T) {
	t.Parallel()
	fixture := loadSessionReindexFixture(t)
	const (
		sessionID   = "6a1f0c7e-2b4d-4e8f-9a3c-5d7e1f2a3b4c"
		neighbourID = "7b2e1d8f-3c5e-4f9a-8b4d-6e8f2a3b4c5d"
		unrecorded  = "8c3f2e9a-4d6f-4a0b-9c5e-7f9a3b4c5d6e"
	)
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			hs := newTestXDGHomes(t)
			root := t.TempDir()
			cfg := config.BaseConfig()
			cfg.Output.BasePath = filepath.Join(root, "managed")
			for harness := range ingest.DefaultAdapterRegistry {
				if provider, ok := cfg.Sources.Provider(harness); ok {
					provider.Enabled = harness == ingest.HarnessClaudeCode
				}
			}
			claude, _ := cfg.Sources.Provider(ingest.HarnessClaudeCode)
			claude.Paths = []string{filepath.Join(root, "claude")}
			project := filepath.Join(root, "claude", "-workspace")
			if err := os.MkdirAll(project, 0o700); err != nil {
				t.Fatal(err)
			}
			sessionPath := filepath.Join(project, sessionID+".jsonl")
			neighbourPath := filepath.Join(project, neighbourID+".jsonl")
			session := fixture.turns(sessionID, 0, c.Turns/2) + strings.ReplaceAll(fixture.SurrogateRecord, "SESSION_ID", sessionID) + fixture.turns(sessionID, c.Turns/2, c.Turns)
			if err := os.WriteFile(sessionPath, []byte(session), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(neighbourPath, []byte(fixture.turns(neighbourID, 0, c.NeighbourTurns)), 0o600); err != nil {
				t.Fatal(err)
			}

			handler := hs.handler(nil, cfg)
			handler.runIngestPipeline(ingest.NewProgressState())
			if handler.ingestError != nil {
				t.Fatalf("whole-machine web ingest: %s", handler.ingestError)
			}
			handler.ingestStatus = ""
			db, err := store.Open(hs.dbPath(), store.WithSkipMigrations())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			handler.store = db

			// The recordings move on after the index was written: the
			// session gains turns (and maybe a broken or half-written
			// record), and so does its neighbour.
			if c.AppendMalformed {
				appendToFile(t, sessionPath, strings.ReplaceAll(fixture.MalformedRecord, "SESSION_ID", sessionID))
			}
			appendToFile(t, sessionPath, fixture.turns(sessionID, c.Turns, c.Turns+c.AppendTurns))
			if c.AppendPartial {
				appendToFile(t, sessionPath, strings.ReplaceAll(fixture.PartialRecord, "SESSION_ID", sessionID))
			}
			appendToFile(t, neighbourPath, fixture.turns(neighbourID, c.NeighbourTurns, c.NeighbourTurns+c.NeighbourAppendTurns))

			if c.Busy {
				handler.ingestStatus = "running"
			}
			if c.Timeout != "" {
				timeout, err := time.ParseDuration(c.Timeout)
				if err != nil {
					t.Fatal(err)
				}
				handler.sessionIngestTimeout = timeout
			}
			target := sessionID
			if c.Unrecorded {
				target = unrecorded
			}
			response := httptest.NewRecorder()
			handler.handleSyncIngestSession(response, httptest.NewRequest(http.MethodPost, "/api/v1/sync/ingest/session?session_id="+target, nil))
			if response.Code != c.Status {
				t.Fatalf("status = %d, want %d; body %s", response.Code, c.Status, response.Body.String())
			}
			var body struct {
				Status string   `json:"status"`
				Notes  []string `json:"notes"`
				Error  string   `json:"error"`
				Code   string   `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", response.Body.String(), err)
			}
			if c.Status == http.StatusOK && body.Status != "done" {
				t.Fatalf("status field = %q, want done", body.Status)
			}
			said := body.Error
			if c.Status == http.StatusOK {
				said = strings.Join(body.Notes, "\n")
			}
			if body.Code != c.Code || !strings.Contains(said, c.Message) {
				t.Fatalf("code %q said %q, want code %q containing %q", body.Code, said, c.Code, c.Message)
			}
			if handler.sessionIngest {
				t.Fatal("the run left the session re-index marked running")
			}
			if got := userTurns(t, db, sessionID); got != c.UserTurnsAfter {
				t.Fatalf("session user turns = %d, want %d", got, c.UserTurnsAfter)
			}
			if got := userTurns(t, db, neighbourID); got != c.NeighbourUserTurnsAfter {
				t.Fatalf("neighbour user turns = %d, want %d: the run reached past its session", got, c.NeighbourUserTurnsAfter)
			}
			if c.ScanStatus == 0 {
				return
			}
			scan := httptest.NewRecorder()
			handler.handleSyncRedactions(scan, httptest.NewRequest(http.MethodGet, "/api/v1/sync/redactions?session_id="+sessionID, nil))
			if scan.Code != c.ScanStatus {
				t.Fatalf("scan status = %d, want %d; body %s", scan.Code, c.ScanStatus, scan.Body.String())
			}
			var result schema.SyncRedactionsResponse
			if err := json.Unmarshal(scan.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Total == 0 {
				t.Fatalf("scan found nothing in a session that names an email address: %s", scan.Body.String())
			}
		})
	}
}
