package village

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/peasant-labs/schema"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/observation_reads.yaml
var observationReadYAML []byte

type observationReadFixture struct {
	Name         string `yaml:"name"`
	Method       string `yaml:"method"`
	Path         string `yaml:"path"`
	Body         string `yaml:"body"`
	Status       int    `yaml:"status"`
	PaddingBytes int    `yaml:"paddingBytes"`
	Invalid      bool   `yaml:"invalid"`
}

func loadObservationReadFixtures(t *testing.T) []observationReadFixture {
	t.Helper()
	var data struct {
		Cases []observationReadFixture `yaml:"cases"`
	}
	if err := yaml.Unmarshal(observationReadYAML, &data); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, fixture := range data.Cases {
		if fixture.Name == "" || names[fixture.Name] {
			t.Fatal("missing/duplicate observation fixture name")
		}
		names[fixture.Name] = true
	}
	const required = "verified-stable-user-uuid username-without-stable-id-refused foreign-viewer-dedicated-associations-only known-empty-complete-associations missing-associations-not-empty null-associations-not-empty repeated-association-refused known-association-nondisclosure-preserves-status available-owned-pair-discovery cross-collective-pair-refused unknown-submission-status-refused null-discovery-not-complete complete-known-empty-discovery truncated-valid-json-prefix trailing-second-json-document-refused"
	for _, name := range strings.Fields(required) {
		if !names[name] {
			t.Fatalf("required fixture %s missing", name)
		}
	}
	return data.Cases
}

func TestObservationSourceCompleteReads(t *testing.T) {
	for _, fixture := range loadObservationReadFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != fixture.Path || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Errorf("unexpected source request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				status := fixture.Status
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, fixture.Body)
				if fixture.PaddingBytes != 0 {
					_, _ = io.WriteString(w, strings.Repeat(" ", fixture.PaddingBytes))
				}
			}))
			defer server.Close()
			client := NewVillageClient(server.URL, "fixture-key", server.Client())
			var err error
			switch fixture.Method {
			case "user":
				_, err = client.VerifyUser(context.Background())
			case "associations":
				_, err = client.ReadTranscriptCollectives(context.Background(), schema.TranscriptID("22222222-2222-4222-8222-222222222222"))
			case "submissions":
				_, err = client.ReadOwnSubmissions(context.Background(), schema.VillageUUID("33333333-3333-4333-8333-333333333333"))
			default:
				t.Fatalf("unknown method %s", fixture.Method)
			}
			if (err != nil) != fixture.Invalid {
				t.Fatalf("invalid=%v err=%v", fixture.Invalid, err)
			}
			if fixture.Status != 0 {
				var status *StatusError
				if !errors.As(err, &status) || status.StatusCode != fixture.Status {
					t.Fatalf("lost nondisclosure status: %v", err)
				}
			}
			if requests.Load() != 1 {
				t.Fatal("source read unexpectedly probed another endpoint")
			}
		})
	}
}

func TestObservationReadDoesNotForwardCredentialsOnRedirect(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(true)
		_, _ = io.WriteString(w, `{"id":"11111111-1111-4111-8111-111111111111"}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/other-installation/api/v1/auth/me", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	_, err := NewVillageClient(source.URL, "fixture-key", source.Client()).VerifyUser(context.Background())
	var status *StatusError
	if !errors.As(err, &status) || status.StatusCode != http.StatusTemporaryRedirect || leaked.Load() {
		t.Fatalf("redirect followed or status lost: leak=%v err=%v", leaked.Load(), err)
	}
}
