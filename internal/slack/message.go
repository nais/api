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

	var headerText string
	switch input.Type {
	case model.FeedbackTypeBug:
		headerText = ":bug: Bug report"
	case model.FeedbackTypeChangeRequest:
		headerText = ":bulb: Change request"
	case model.FeedbackTypeOther:
		headerText = ":speech_balloon: Other feedback"
	case model.FeedbackTypeQuestion:
		headerText = ":question: Question"
	}

	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, false, false))

	blocks = append(blocks, headerBlock)

	var userBlock *slack.SectionBlock

	if user != "" {
		userBlock = slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*From:* %s", user), false, false), nil, nil)
		blocks = append(blocks, userBlock)
	}

	url := fmt.Sprintf("https://console.%s.cloud.nais.io", tenant) + input.URI
	uriBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*URL:* %s", url), false, false), nil, nil)
	tenantBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s", tenant), false, false), nil, nil)
	textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", input.Details, false, false), nil, nil)
	blocks = append(blocks, uriBlock, tenantBlock, textBlock)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
