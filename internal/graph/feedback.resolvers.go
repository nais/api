package graph

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateFeedback(ctx context.Context, input model.CreateFeedbackInput) (*model.CreateFeedbackResult, error) {
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
