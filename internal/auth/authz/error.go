package authz

import (
	"fmt"
)

type ErrMissingAuthorization struct {
	authorization string
}

func (e ErrMissingAuthorization) Error() string {
	return fmt.Sprintf("required authorization: %q", e.authorization)
}

func (e ErrMissingAuthorization) GraphError() string {
	return fmt.Sprintf("You are authenticated, but your account is not authorized to perform this action. Specifically, you need the %q authorization.", e.authorization)
}
