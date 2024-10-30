package model

import (
	"strings"

	"k8s.io/utils/ptr"
)

func (input UpdateTeamInput) Sanitize() UpdateTeamInput {
	if input.Purpose != nil {
		input.Purpose = ptr.To(strings.TrimSpace(*input.Purpose))
	}

	if input.SlackChannel != nil {
		input.SlackChannel = ptr.To(strings.TrimSpace(*input.SlackChannel))
	}
	return input
}

func (input UpdateTeamSlackAlertsChannelInput) Sanitize() UpdateTeamSlackAlertsChannelInput {
	input.Environment = strings.TrimSpace(input.Environment)
	if input.ChannelName != nil {
		input.ChannelName = ptr.To(strings.TrimSpace(*input.ChannelName))
	}

	return input
}
