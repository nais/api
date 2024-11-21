package serviceaccount

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

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
