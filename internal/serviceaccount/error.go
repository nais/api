package serviceaccount

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrUIDMismatch is returned when an authentication attempt is made with a Kubernetes ServiceAccount UID that does
// not match the UID stored on the binding (after trust-on-first-use).
var ErrUIDMismatch = errors.New("kubernetes service account uid mismatch")

type ErrNotFound struct {
	err error
}

func (e ErrNotFound) Error() string {
	err := "service account not found"
	if e.err != nil {
		err += ": " + e.err.Error()
	}
	return err
}

func (e ErrNotFound) GraphError() string {
	return "The specified service account was not found."
}

func (e *ErrNotFound) As(v any) bool {
	_, ok := v.(*ErrNotFound)
	return ok
}

func (e *ErrNotFound) Is(v error) bool {
	_, ok := v.(*ErrNotFound)
	return ok
}

func handleError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound{
			err: err,
		}
	}
	return err
}

type ErrBindingNotFound struct {
	err error
}

func (e ErrBindingNotFound) Error() string {
	err := "service account workload binding not found"
	if e.err != nil {
		err += ": " + e.err.Error()
	}
	return err
}

func (e ErrBindingNotFound) GraphError() string {
	return "The specified service account workload binding was not found."
}

func (e *ErrBindingNotFound) As(v any) bool {
	_, ok := v.(*ErrBindingNotFound)
	return ok
}

func (e *ErrBindingNotFound) Is(v error) bool {
	_, ok := v.(*ErrBindingNotFound)
	return ok
}

func handleBindingError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBindingNotFound{err: err}
	}
	return err
}
