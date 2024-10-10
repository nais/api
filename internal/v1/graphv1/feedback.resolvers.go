package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/feedback"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) ReportConsoleUserFeedback(ctx context.Context, input feedback.ReportConsoleUserFeedbackInput) (*feedback.ReportConsoleUserFeedbackPayload, error) {
	if err := feedback.Create(ctx, &input); err != nil {
		return nil, err
	}

	return &feedback.ReportConsoleUserFeedbackPayload{
		Reported: ptr.To(true),
	}, nil
}
