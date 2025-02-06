package authz

import (
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
)

var ErrUnauthorized = apierror.Errorf("You are authenticated, but your account is not authorized to perform this action.")

type ErrMissingAuthorization struct {
	missingAuthorization string
}

func (e ErrMissingAuthorization) Error() string {
	return fmt.Sprintf("required authorization missing: %q", e.missingAuthorization)
}

func (e ErrMissingAuthorization) GraphError() string {
	return fmt.Sprintf("You are authenticated, but your account is not authorized to perform this action. Specifically, you need the %q authorization.", e.missingAuthorization)
}

func newMissingAuthorizationError(missingAuthorization string) error {
	return ErrMissingAuthorization{
		missingAuthorization: missingAuthorization,
	}
}
