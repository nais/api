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
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	ErrInternal                        = Errorf("The server errored out while processing your request, and we didn't write a suitable error message. You might consider that a bug on our side. Please try again, and if the error persists, contact the NAIS team.")
	ErrDatabase                        = Errorf("The database encountered an error while processing your request. This is probably a transient error, please try again. If the error persists, contact the NAIS team.")
	ErrAppNotFound                     = Errorf("We were unable to find the app you were looking for.")
	ErrUserNotExists                   = Errorf("The user does not exist.")
	ErrTeamSlug                        = Errorf("Your team identifier does not fit our requirements. Team identifiers must contain only lowercase alphanumeric characters or hyphens, contain at least 3 characters and at most 30 characters, start with an alphabetic character, end with an alphanumeric character, and not contain two hyphens in a row.")
	ErrTeamPurpose                     = Errorf("You must specify the purpose for your team. This is a human-readable string which is used in external systems, and is important because other people might need to to understand what your team is all about.")
	ErrTeamNotExist                    = Errorf("The team you are referring to does not exist in our database.")
	ErrTeamPrefixRedundant             = Errorf("The name prefix 'team' is redundant. When you create a team, it is by definition a team. Try again with a different name, perhaps just removing the prefix?")
	ErrTeamSlugReserved                = Errorf("The specified slug is reserved by the platform.")
	ErrUserIsNotTeamMember             = Errorf("The user is not a member of the team.")
	ErrSecretUnmanaged                 = Errorf("The secret name conflicts with a secret managed by another service. Please try again with a different name.")
	ErrGoogleCloudMonitoringMetricsApi = Errorf("Unable to fetch SQL instance metrics from the Google Cloud Monitoring API")
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
// All filtered errors are logged.
func GetErrorPresenter(log logrus.FieldLogger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)
		unwrappedError := errors.Unwrap(e)

		switch originalError := unwrappedError.(type) {
		case Error:
			// Error is already formatted for end-user consumption.
			return err
		case authz.ErrMissingRole:
			err.Message = fmt.Sprintf("You are authenticated, but your account is not authorized to perform this action. Specifically, you need the %q role.", originalError.Role())
			return err
		case authz.ErrMissingAuthorization:
			err.Message = fmt.Sprintf("You are authenticated, but your account is not authorized to perform this action. Specifically, you need the %q authorization.", originalError.Authorization())
			return err
		case *pgconn.PgError:
			err.Message = ErrDatabase.Error()
			log.WithError(originalError).Errorf("database error %s: %s (%s)", originalError.Code, originalError.Message, originalError.Detail)
			return err
		default:
			break
		}

		switch unwrappedError {
		case sql.ErrNoRows, pgx.ErrNoRows:
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
			err.Message = ErrInternal.Error()
		}

		return err
	}
}
