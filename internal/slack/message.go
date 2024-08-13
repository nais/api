package slack

import (
	"fmt"

	"github.com/slack-go/slack"
)

func (s *Slack) GetFeedbackMessageOptions(tenant, user, uri, text string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":feedback: Console user feedback", false, false))

	blocks = append(blocks, headerBlock)

	var userBlock *slack.SectionBlock

	if user != "" {
		userBlock = slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*User:* %s", user), false, false), nil, nil)
		blocks = append(blocks, userBlock)
	}

	uriBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*URI:* %s", uri), false, false), nil, nil)
	tenantBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s", tenant), false, false), nil, nil)
	textBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Feedback:* %s", text), false, false), nil, nil)
	blocks = append(blocks, uriBlock, tenantBlock, textBlock)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
