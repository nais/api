package authz

import (
	"fmt"
)

type ErrMissingRole struct {
	role string
}

func (e ErrMissingRole) Error() string {
	return fmt.Sprintf("required role: %q", e.role)
}

func (e ErrMissingRole) Role() string {
	return e.role
}

type ErrMissingAuthorization struct {
	authorization string
}

func (e ErrMissingAuthorization) Error() string {
	return fmt.Sprintf("required authorization: %q", e.authorization)
}

func (e ErrMissingAuthorization) Authorization() string {
	return e.authorization
}
