package feedback

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
)

func Create(ctx context.Context, input *ReportConsoleUserFeedbackInput) error {
	input = input.Sanitized(authz.ActorFromContext(ctx))

	if input.Feedback == "" {
		return apierror.Errorf("You must provide a feedback message.")
	}

	if len(input.Feedback) > 3000 {
		return apierror.Errorf("The content of the feedback message must not exceed 3000 characters.")
	}

	return fromContext(ctx).feedbackClient.PostFeedback(ctx, input)
}
