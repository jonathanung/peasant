package collectiveobservations

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peasant-labs/peasant/internal/auth"
	"github.com/peasant-labs/peasant/internal/store/storetest"
	"github.com/peasant-labs/schema"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/fetch.yaml
var fetchFixtureYAML []byte

type fetchCase struct {
	Name               string                    `yaml:"name"`
	MembershipEmpty    bool                      `yaml:"membershipEmpty"`
	ContributionEmpty  bool                      `yaml:"contributionEmpty"`
	Org                bool                      `yaml:"org"`
	Track              bool                      `yaml:"track"`
	FailPath           string                    `yaml:"failPath"`
	FailStatus         int                       `yaml:"failStatus"`
	Cancel             bool                      `yaml:"cancel"`
	MissingCounter     bool                      `yaml:"missingCounter"`
	AssociationFailure bool                      `yaml:"associationFailure"`
	WantStatus         int                       `yaml:"wantStatus"`
	WantName           string                    `yaml:"wantName"`
	WantEvent          schema.VillageShareStatus `yaml:"wantEvent"`
}

func TestFetchIndependentCompleteDomains(t *testing.T) {
	var fixtures struct {
		RequiredNames []string    `yaml:"requiredNames"`
		Cases         []fetchCase `yaml:"cases"`
	}
	d := yaml.NewDecoder(strings.NewReader(string(fetchFixtureYAML)))
	d.KnownFields(true)
	if err := d.Decode(&fixtures); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range fixtures.Cases {
		if c.Name == "" || names[c.Name] {
			t.Fatal("missing/duplicate fetch case")
		}
		names[c.Name] = true
	}
	for _, name := range fixtures.RequiredNames {
		if !names[name] {
			t.Fatalf("required fetch case %s absent", name)
		}
	}
	for _, c := range fixtures.Cases {
		t.Run(c.Name, func(t *testing.T) {
			db := storetest.Open(t)
			home := t.TempDir()
			var phase atomic.Int32
			operation, stop := context.WithCancel(t.Context())
			defer stop()
			const collective = "33333333-3333-4333-8333-333333333333"
			const transcript = "22222222-2222-4222-8222-222222222222"
			creds := auth.Credentials{APIKey: "fixture-key", KeyID: "fixture-key-id", UserID: "11111111-1111-4111-8111-111111111111", Username: "fixture"}
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Errorf("non-metadata or wrong-credential request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(400)
					return
				}
				current := phase.Load() > 0
				name := "Old"
				if current {
					name = "New"
				}
				if current && c.FailPath == r.URL.Path {
					if c.Cancel {
						stop()
						<-r.Context().Done()
						return
					}
					w.WriteHeader(c.FailStatus)
					_, _ = w.Write([]byte(`{"error":"fixture unavailable"}`))
					return
				}
				var value any
				switch r.URL.Path {
				case "/api/v1/auth/me":
					value = schema.VillageUser{ID: schema.VillageUUID(creds.UserID)}
				case "/api/v1/transcripts/" + transcript + "/collectives":
					value = schema.VillageTranscriptCollectivesResponse{Collectives: []schema.VillageTranscriptCollective{}}
				case "/api/v1/transcripts/44444444-4444-4444-8444-444444444444/collectives":
					if current && c.AssociationFailure {
						stop()
						<-r.Context().Done()
						return
					}
					value = schema.VillageTranscriptCollectivesResponse{Collectives: []schema.VillageTranscriptCollective{}}
				case "/api/v1/collectives":
					rows := []schema.VillageUserGroup{}
					if !current || !c.MembershipEmpty {
						rows = append(rows, schema.VillageUserGroup{ID: collective, Name: name, Role: "member", AcceptanceMode: "open", DataAccess: "public", MemberSince: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
					}
					value = rows
				case "/api/v1/users/me/collectives/contributions":
					if current && c.MissingCounter {
						_, _ = w.Write([]byte(`{"collectives":[{"id":"` + collective + `","name":"New","description":null,"linked_github_org":null,"approved_count":0,"rejected_attempt_count":0,"withdrawn_attempt_count":0}]}`))
						return
					}
					rows := []schema.VillageContributedCollective{}
					if !current || !c.ContributionEmpty {
						rows = append(rows, schema.VillageContributedCollective{ID: collective, Name: name, PendingCount: 1})
					}
					value = schema.VillageContributedCollectivesResponse{Collectives: rows}
				case "/api/v1/auth/orgs":
					rows := []schema.VillageMetadataUserOrganization{}
					if c.Org {
						rows = append(rows, schema.VillageMetadataUserOrganization{OrgID: 1, OrgLogin: "example", FetchedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
					}
					value = rows
				case "/api/v1/orgs/example":
					value = schema.VillageOrganizationCollectivesResponse{Collectives: []schema.VillageOrganizationCollective{{ID: collective, Name: name}}}
				case "/api/v1/collectives/" + collective:
					value = schema.VillageGroupDetailResponse{Group: schema.VillageGroupDetailRecord{ID: collective, Name: name, AcceptanceMode: "open", DataAccess: "public"}}
				case "/api/v1/users/me/collectives/" + collective + "/submissions":
					value = []schema.VillageCollectiveSubmission{{TranscriptID: transcript, GroupID: collective, EventNum: 1, Status: "pending", RecordedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
				case "/api/v1/users/me/collectives/" + collective + "/transcripts/" + transcript + "/events":
					event := schema.VillageShareEvent{EventNum: 1, Status: "pending", RecordedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
					if current {
						at := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
						event.Status = "approved"
						event.DecidedAt = &at
						event.DecidedByActor = "moderator"
					}
					value = []schema.VillageShareEvent{event}
				default:
					t.Errorf("unexpected non-narrow metadata route: %s", r.URL.String())
					w.WriteHeader(404)
					return
				}
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer remote.Close()
			creds.VillageURL = remote.URL
			if err := auth.SaveCredentialsFrom(&creds, home); err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
			service := New(Dependencies{Store: db, ConfigHome: home, Now: func() time.Time { return at }, LocallyHeldTranscriptIDs: func(context.Context, schema.CollectiveObservationScope) ([]schema.TranscriptID, error) {
				if c.AssociationFailure {
					return []schema.TranscriptID{transcript, "44444444-4444-4444-8444-444444444444"}, nil
				}
				return []schema.TranscriptID{}, nil
			}})
			if c.Track {
				if _, err := service.Track(t.Context(), collective); err != nil {
					t.Fatal(err)
				}
			}
			before, err := service.Fetch(t.Context())
			if err != nil {
				t.Fatalf("initial full fetch: %v %#v", err, before)
			}
			phase.Store(1)
			at = at.Add(time.Hour)
			report, err := service.Fetch(operation)
			if (err != nil) != (c.WantStatus != 200) {
				t.Fatalf("report error=%v status=%d domains=%#v", err, schema.CollectiveFetchHTTPStatus(report), report.Domains)
			}
			if validation := report.Validate(); validation != nil {
				t.Fatalf("invalid typed report: %v %#v", validation, report)
			}
			if c.AssociationFailure {
				domain := report.Domains[2]
				if domain.Commit != schema.CollectiveFetchCommitUpdated || domain.Associations.Completed != 1 || domain.Associations.Failed != 1 || domain.Associations.Scopes[0].State != schema.CollectiveFetchStateComplete || domain.Associations.Scopes[1].State == schema.CollectiveFetchStateComplete {
					t.Fatalf("per-set effects were lost: %#v", domain)
				}
			}
			if status := schema.CollectiveFetchHTTPStatus(report); status != c.WantStatus {
				t.Fatalf("status=%d want=%d report=%#v", status, c.WantStatus, report)
			}
			catalog, err := service.List(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(catalog.Collectives) != 1 || catalog.Collectives[0].Name != c.WantName {
				t.Fatalf("catalog=%#v want=%s", catalog.Collectives, c.WantName)
			}
			history, err := service.History(t.Context(), schema.LocalContributionHistoryQuery{Kind: schema.LocalContributionEvidenceRemoteAttempts, Page: 1, Limit: 50})
			if err != nil {
				t.Fatal(err)
			}
			if len(history.RemoteAttempts) != 1 || history.RemoteAttempts[0].Attempt.Status != c.WantEvent {
				t.Fatalf("history=%#v want=%s", history.RemoteAttempts, c.WantEvent)
			}
			if c.WantStatus != 200 && catalog.Fetch.LastCompletedFetchAt != nil && !catalog.Fetch.LastCompletedFetchAt.Equal(before.Domains[0].CompletedAt.UTC()) {
				t.Fatal("partial fetch advanced all-complete timestamp")
			}
		})
	}
}
