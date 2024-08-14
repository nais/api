package slack

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/slack-go/slack"
)

func (s *Slack) GetFeedbackMessageOptions(ctx context.Context, tenant string, input model.CreateFeedbackInput) []slack.MsgOption {
	user := "anonymous"

	if !input.Anonymous {
		actor := authz.ActorFromContext(ctx)
		if actor != nil {
			user = actor.User.Identity()
		}
	}

	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":feedback: Console user feedback - "+input.Type.String(), false, false))

	blocks = append(blocks, headerBlock)

	var userBlock *slack.SectionBlock

	if user != "" {
		userBlock = slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*User:* %s", user), false, false), nil, nil)
		blocks = append(blocks, userBlock)
	}

	uriBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*URI:* %s", input.URI), false, false), nil, nil)
	tenantBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s", tenant), false, false), nil, nil)
	textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Details:* %s", input.Details), false, false), nil, nil)
	blocks = append(blocks, uriBlock, tenantBlock, textBlock)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
