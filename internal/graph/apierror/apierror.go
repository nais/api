package apierror

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/validate"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type graphError interface {
	GraphError() string
}

// Error is an error that can be presented to end-users
type Error struct {
	err error
}

// Error returns the formatted message for end-users
func (e Error) Error() string {
	return e.err.Error()
}

func (e Error) GraphError() string {
	return e.err.Error()
}

// Errorf formats an error message for end-users. Remember not to leak sensitive information in error messages.
func Errorf(format string, args ...any) Error {
	return Error{
		err: fmt.Errorf(format, args...),
	}
}

// GetErrorPresenter returns a GraphQL error presenter that filters out error messages not intended for end users.
// All filtered errors are logged.
func GetErrorPresenter(log logrus.FieldLogger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)
		unwrappedError := errors.Unwrap(e)

		if unwrappedError == nil {
			return err
		}

		switch originalError := unwrappedError.(type) {
		case *gqlerror.Error:
			return originalError
		case graphError:
			err.Message = originalError.GraphError()
			return err
		case *pgconn.PgError:
			err.Message = "The database encountered an error while processing your request. This is probably a transient error, please try again. If the error persists, contact the NAIS team."
			log.WithError(originalError).Errorf("database error %s: %s (%s)", originalError.Code, originalError.Message, originalError.Detail)
			return err
		case *validate.ValidationErrors:
			var verr *gqlerror.Error
			// Add errors in the correct order. The returned error will be the last one.
			for i, err := range originalError.Errors {
				if i > 0 {
					graphql.AddError(ctx, verr)
				}
				verr = &gqlerror.Error{
					Message: err.Message,
					Path:    graphql.GetPath(ctx),
				}
				if err.GraphQLField != nil {
					verr.Extensions = map[string]any{
						"field": err.GraphQLField,
					}
				}
			}
			return verr

		default:
			break
		}

		switch unwrappedError {
		case sql.ErrNoRows, pgx.ErrNoRows, loader.ErrObjectNotFound:
			err.Message = "Object was not found in the database. This usually means you specified a non-existing team identifier or e-mail address."
		case authz.ErrNotAuthenticated:
			err.Message = "Valid user required. You are not logged in."
		case context.Canceled:
			// This won't make it back to the caller if they have cancelled the request on their end
			err.Message = "Request canceled"
		default:
			identity := "<unknown>"
			actor := authz.ActorFromContext(ctx)
			if actor != nil {
				identity = actor.User.Identity()
			}
			log.WithError(err).WithField("actor", identity).Errorf("unhandled error: %q", err)
			err.Message = "The server errored out while processing your request, and we didn't write a suitable error message. You might consider that a bug on our side. Please try again, and if the error persists, contact the NAIS team."
		}

		return err
	}
}
