package elevation

import "github.com/nais/api/internal/graph/apierror"

var (
	ErrEnvironmentNotFound = apierror.Errorf("Environment does not exist.")
	ErrInvalidDuration     = apierror.Errorf("Duration must be between 1 and 60 minutes.")
	ErrReasonTooShort      = apierror.Errorf("Reason must be at least 10 characters.")
	ErrNotAuthorized       = apierror.Errorf("You are not authorized to perform this action.")
)
