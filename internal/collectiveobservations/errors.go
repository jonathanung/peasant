package collectiveobservations

import (
	"context"
	"errors"

	"github.com/peasant-labs/peasant/internal/auth"
	"github.com/peasant-labs/peasant/internal/store"
)

type observationPersistenceError struct{ error }

func (e *observationPersistenceError) Unwrap() error { return e.error }

func IsSourceContractError(err error) bool {
	var contract *observationSourceContractError
	return errors.As(err, &contract)
}

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	var conflict *store.ContributionAttemptConflictError
	if errors.Is(err, auth.ErrCredentialsChanged) || errors.Is(err, auth.ErrSignedOut) || errors.Is(err, store.ErrObservationSuperseded) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &conflict) {
		return err
	}
	return &observationPersistenceError{error: err}
}
