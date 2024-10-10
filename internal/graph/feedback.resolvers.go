package graph

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/v1/feedback"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateFeedback(ctx context.Context, input model.CreateFeedbackInput) (*model.CreateFeedbackResult, error) {
	if len(input.Details) == 0 {
		return &model.CreateFeedbackResult{
			Created: false,
			Error:   ptr.To("Feedback details are required"),
		}, nil
	}
	if len(input.Details) > 3000 {
		return &model.CreateFeedbackResult{
			Created: false,
			Error:   ptr.To("Feedback details must be no more than 3000 characters"),
		}, nil
	}

	i := &feedback.ReportConsoleUserFeedbackInput{
		Feedback:  input.Details,
		Path:      input.URI,
		Anonymous: input.Anonymous,
		Type:      feedback.ConsoleUserFeedbackType(input.Type),
	}
	if err := r.feedbackClient.PostFeedback(ctx, i); err != nil {
		return &model.CreateFeedbackResult{
			Created: false,
			Error:   ptr.To(err.Error()),
		}, err
	}

	return &model.CreateFeedbackResult{
		Created: true,
		Error:   nil,
	}, nil
}
