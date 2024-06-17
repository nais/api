package model

import (
	"strings"

	"k8s.io/utils/ptr"
)

func (input CreateTeamInput) Sanitize() CreateTeamInput {
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.SlackChannel = strings.TrimSpace(input.SlackChannel)
	return input
}

func (input SlackAlertsChannelInput) Sanitize() SlackAlertsChannelInput {
	input.Environment = strings.TrimSpace(input.Environment)
	if input.ChannelName != nil {
		input.ChannelName = ptr.To(strings.TrimSpace(*input.ChannelName))
	}

	return input
}
