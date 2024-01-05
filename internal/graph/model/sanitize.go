package model

import (
	"strings"
)

func (input CreateTeamInput) Sanitize() CreateTeamInput {
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.SlackChannel = strings.TrimSpace(input.SlackChannel)
	return input
}

func (input UpdateTeamInput) Sanitize() UpdateTeamInput {
	if input.Purpose != nil {
		input.Purpose = ptr(strings.TrimSpace(*input.Purpose))
	}

	if input.SlackChannel != nil {
		input.SlackChannel = ptr(strings.TrimSpace(*input.SlackChannel))
	}

	channels := make([]*SlackAlertsChannelInput, len(input.SlackAlertsChannels))
	for i := range input.SlackAlertsChannels {
		channel := *input.SlackAlertsChannels[i]
		channel.Environment = strings.TrimSpace(channel.Environment)
		if channel.ChannelName != nil {
			channel.ChannelName = ptr(strings.TrimSpace(*channel.ChannelName))
		}
		channels[i] = &channel
	}
	input.SlackAlertsChannels = channels

	return input
}
