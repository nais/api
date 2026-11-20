package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func backendError(operation string, err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return apierror.Errorf("%s", status.Convert(err).Message())
	case codes.NotFound:
		return apierror.ErrNotFound
	case codes.Unavailable:
		return apierror.Errorf("Vulnerability data is temporarily unavailable. Please try again later.")
	case codes.DeadlineExceeded:
		return apierror.Errorf("The vulnerability request timed out. Please try again.")
	case codes.Canceled:
		return context.Canceled
	default:
		return fmt.Errorf("%s: %w", operation, err)
	}
}
