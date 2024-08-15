package graph

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateFeedback(ctx context.Context, input model.CreateFeedbackInput) (*model.CreateFeedbackResult, error) {
	if len(input.Details) == 0 {
		return &model.CreateFeedbackResult{
			Created: false,
			Error:   ptr.To("Feedback details are required"),
		}, nil
	}
	r.log.Infof("Creating feedback of length %d", len(input.Details))
	if len(input.Details) > 3000 {
		return &model.CreateFeedbackResult{
			Created: false,
			Error:   ptr.To("Feedback details must be no more than 3000 characters"),
		}, nil
	}

	messageOptions := r.slackClient.GetFeedbackMessageOptions(ctx, r.tenant, input)
	if _, _, err := r.slackClient.PostFeedbackMessage(messageOptions); err != nil {
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
