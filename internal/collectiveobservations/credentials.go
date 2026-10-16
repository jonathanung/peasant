// Package collectiveobservations coordinates account-scoped local evidence.
package collectiveobservations

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/peasant-labs/peasant/internal/auth"
	"github.com/peasant-labs/peasant/internal/store"
	"github.com/peasant-labs/peasant/internal/village"
	"github.com/peasant-labs/schema"
)

// IdentityReader verifies the stable caller of the selected Village credential.
// It is a transport dependency, not an authorization rule implemented locally.
type IdentityReader interface {
	VerifyUser(context.Context) (schema.VillageUser, error)
}

var _ IdentityReader = (*village.VillageClient)(nil)

// Dependencies selects the credential profile and remote identity transport.
type Dependencies struct {
	ConfigHome        string
	NewIdentityReader func(auth.Credentials) IdentityReader
	Store             *store.Store
	Now               func() time.Time
}

// Service shares credential selection between offline readers and online
// observation operations. A snapshot is never itself remote authorization.
type Service struct {
	configHome        string
	newIdentityReader func(auth.Credentials) IdentityReader
	store             *store.Store
	now               func() time.Time
}

// New uses the real Village transport unless a dependency is explicitly supplied.
func New(deps Dependencies) *Service {
	factory := deps.NewIdentityReader
	if factory == nil {
		factory = func(creds auth.Credentials) IdentityReader {
			return village.NewVillageClient(creds.VillageURL, creds.APIKey, nil)
		}
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Service{configHome: deps.ConfigHome, newIdentityReader: factory, store: deps.Store, now: now}
}

// SelectedCredentials selects the saved private scope OFFLINE. It never logs
// in, verifies remotely, or falls back to the most recently used account.
func (s *Service) SelectedCredentials(ctx context.Context) (auth.CredentialSnapshot, error) {
	snapshot, err := auth.CaptureCredentials(ctx, s.configHome)
	if err != nil {
		return auth.CredentialSnapshot{}, err
	}
	creds := snapshot.Credentials()
	id, err := uuid.Parse(creds.UserID)
	if err != nil || id.String() != creds.UserID {
		return auth.CredentialSnapshot{}, fmt.Errorf("select private collective scope: saved user identity is not a canonical Village UUID; no private observations were selected; sign in again (unfiltered local inventory remains available)")
	}
	if _, err := auth.NormalizeVillageURL(creds.VillageURL); err != nil {
		return auth.CredentialSnapshot{}, err
	}
	return snapshot, nil
}

// AuthenticatedCredentials verifies auth/me using a captured credential outside
// any local write lock. Every later domain commit must still call WithCurrent
// around its actual transaction; this preliminary check cannot replace it.
func (s *Service) AuthenticatedCredentials(ctx context.Context) (auth.CredentialSnapshot, error) {
	snapshot, _, err := s.authenticatedReader(ctx)
	return snapshot, err
}

func (s *Service) authenticatedReader(ctx context.Context) (auth.CredentialSnapshot, IdentityReader, error) {
	snapshot, err := s.SelectedCredentials(ctx)
	if err != nil {
		return auth.CredentialSnapshot{}, nil, err
	}
	creds := snapshot.Credentials()
	creds.VillageURL, err = auth.NormalizeVillageURL(creds.VillageURL)
	if err != nil {
		return auth.CredentialSnapshot{}, nil, err
	}
	reader := s.newIdentityReader(creds)
	if reader == nil {
		return auth.CredentialSnapshot{}, nil, fmt.Errorf("verify collective observation scope: identity transport is missing; no observations were fetched; configure the Village client and retry")
	}
	user, err := reader.VerifyUser(ctx)
	if err != nil {
		return auth.CredentialSnapshot{}, nil, fmt.Errorf("verify collective observation scope before fetching: %w; no new metadata was installed; verify the selected login and retry explicitly", err)
	}
	if user.ID.String() != creds.UserID {
		return auth.CredentialSnapshot{}, nil, fmt.Errorf("verify collective observation scope: auth/me disagrees with the saved stable user UUID; no private metadata was selected or installed; sign in again to select the correct account")
	}
	if err := snapshot.WithCurrent(ctx, func() error { return nil }); err != nil {
		return auth.CredentialSnapshot{}, nil, err
	}
	return snapshot, reader, nil
}
