package collectiveobservations

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/peasant-labs/peasant/internal/auth"
	"github.com/peasant-labs/schema"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/credential_selection.yaml
var credentialSelectionYAML []byte

type credentialSelectionCase struct {
	Name         string `yaml:"name"`
	Online       bool   `yaml:"online"`
	SavedUserID  string `yaml:"savedUserId"`
	RemoteUserID string `yaml:"remoteUserId"`
	Status       int    `yaml:"status"`
	Rotate       bool   `yaml:"rotate"`
	Invalid      bool   `yaml:"invalid"`
	NoRequest    bool   `yaml:"noRequest"`
}

func loadCredentialSelectionFixtures(t *testing.T) []credentialSelectionCase {
	t.Helper()
	var fixtures struct {
		Cases []credentialSelectionCase `yaml:"cases"`
	}
	if err := yaml.Unmarshal(credentialSelectionYAML, &fixtures); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, fixture := range fixtures.Cases {
		if fixture.Name == "" || names[fixture.Name] {
			t.Fatal("missing/duplicate credential fixture name")
		}
		names[fixture.Name] = true
	}
	for _, required := range strings.Fields("offline-no-network online-stable-user-not-username username-reuse invalid-saved-stable-id authentication-refused credentials-rotated-during-authentication") {
		if !names[required] {
			t.Fatalf("missing required fixture %s", required)
		}
	}
	return fixtures.Cases
}

func TestCredentialScopeSelection(t *testing.T) {
	for _, fixture := range loadCredentialSelectionFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			home := t.TempDir()
			var calls atomic.Int32
			var creds auth.Credentials
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/installation/api/v1/auth/me" || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Errorf("wrong selected source %s", r.URL.Path)
				}
				if fixture.Rotate {
					if err := auth.SaveCredentialsFrom(&creds, home); err != nil {
						t.Error(err)
					}
				}
				status := fixture.Status
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(schema.VillageUser{ID: schema.VillageUUID(fixture.RemoteUserID), GithubUsername: "same-username"})
			}))
			defer server.Close()
			creds = auth.Credentials{APIKey: "fixture-key", KeyID: "fixture-key-id", UserID: fixture.SavedUserID, Username: "same-username", VillageURL: server.URL + "/installation/"}
			if err := auth.SaveCredentialsFrom(&creds, home); err != nil {
				t.Fatal(err)
			}
			service := New(Dependencies{ConfigHome: home})
			var err error
			if fixture.Online {
				_, err = service.AuthenticatedCredentials(context.Background())
			} else {
				_, err = service.SelectedCredentials(context.Background())
			}
			if (err != nil) != fixture.Invalid {
				t.Fatalf("invalid=%v err=%v", fixture.Invalid, err)
			}
			if fixture.Rotate && !errors.Is(err, auth.ErrCredentialsChanged) {
				t.Fatalf("credential generation was not checked after auth/me: %v", err)
			}
			wantCalls := int32(1)
			if !fixture.Online || fixture.NoRequest {
				wantCalls = 0
			}
			if calls.Load() != wantCalls {
				t.Fatalf("network requests=%d want=%d", calls.Load(), wantCalls)
			}
		})
	}
}

func TestSignedOutPrivateScopeDoesNotLogin(t *testing.T) {
	service := New(Dependencies{ConfigHome: t.TempDir(), NewIdentityReader: func(auth.Credentials) IdentityReader { t.Fatal("signed-out reader contacted Village"); return nil }})
	if _, err := service.SelectedCredentials(context.Background()); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("signed-out private selection: %v", err)
	}
	if _, err := service.AuthenticatedCredentials(context.Background()); !errors.Is(err, auth.ErrSignedOut) {
		t.Fatalf("signed-out online selection: %v", err)
	}
}
