package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateFeedback(ctx context.Context, input model.CreateFeedbackInput) (*model.CreateFeedbackResult, error) {
	content := input.Content
	user := "anonymous"

	if !input.Anonymous {
		actor := authz.ActorFromContext(ctx)
		if actor != nil {
			user = actor.User.Identity()
		}
	}

	messageOptions := r.slackClient.GetFeedbackMessageOptions(r.tenant, user, input.URI, content)
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
