package apierror

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	ErrInternal         = Errorf("The server errored out while processing your request, and we didn't write a suitable error message. You might consider that a bug on our side. Please try again, and if the error persists, contact the NAIS team.")
	ErrDatabase         = Errorf("The database encountered an error while processing your request. This is probably a transient error, please try again. If the error persists, contact the NAIS team.")
	ErrAppNotFound      = Errorf("We were unable to find the app you were looking for.")
	ErrAppTeamNotFound  = Errorf("NAIS Teams could not find the team which owns the application.")
	ErrTeamNotFound     = Errorf("We were unable to find the team you were looking for.")
	ErrNoEmailInSession = Errorf("No email address found in the session. This is most likely a bug in the backend. Please try again, and if the error persists, contact the NAIS team.")
	ErrUserNotFound     = func(email string) Error {
		return Errorf("We were unable to find a user with the email address you are currently signed in with: %q", email)
	}
)

// Error is an error that can be presented to end-users
type Error struct {
	err error
}

// Error returns the formatted message for end-users
func (e Error) Error() string {
	return e.err.Error()
}

// Errorf formats an error message for end-users. Remember not to leak sensitive information in error messages.
func Errorf(format string, args ...any) Error {
	return Error{
		err: fmt.Errorf(format, args...),
	}
}

// GetErrorPresenter returns a GraphQL error presenter that filters out error messages not intended for end users.
// Filtered errors will be logged with the original error attached.
func GetErrorPresenter(log logrus.FieldLogger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		gqlError := graphql.DefaultErrorPresenter(ctx, err)
		unwrappedError := errors.Unwrap(err)

		switch originalError := unwrappedError.(type) {
		default:
			break
		case Error:
			return gqlError // err is already formatted for end-user
		case *pgconn.PgError:
			gqlError.Message = ErrDatabase.Error()
			log.WithError(originalError).Errorf("database error")
			return gqlError
		}

		switch {
		default:
			log.WithError(unwrappedError).Errorf("unhandled error in the GraphQL error presenter")
			gqlError.Message = ErrInternal.Error()
		case errors.Is(unwrappedError, sql.ErrNoRows):
			gqlError.Message = "Object was not found in the database."
		case errors.Is(unwrappedError, context.Canceled):
			gqlError.Message = "Request canceled."
		}

		return gqlError
	}
}
