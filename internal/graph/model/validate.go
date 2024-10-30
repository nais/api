package model

import (
	"regexp"
	"strings"

	"github.com/nais/api/internal/graph/apierror"
)

// Rules can be found here: https://api.slack.com/methods/conversations.create#naming
var slackChannelNameRegex = regexp.MustCompile("^#[a-z0-9æøå_-]{2,80}$")

func (input UpdateTeamSlackAlertsChannelInput) Validate(validEnvironments []string) error {
	validEnvironment := func(env string) bool {
		for _, environment := range validEnvironments {
			if env == environment {
				return true
			}
		}
		return false
	}

	if !validEnvironment(input.Environment) {
		return apierror.Errorf("The specified environment is not valid: %q. Valid environments are: %s.", input.Environment, strings.Join(validEnvironments, ", "))
	}

	if input.ChannelName != nil && !slackChannelNameRegex.MatchString(*input.ChannelName) {
		return slackChannelError(*input.ChannelName)
	}

	return nil
}

func slackChannelError(channel string) apierror.Error {
	return apierror.Errorf("The Slack channel does not fit the requirements: %q. The name must contain at least 2 characters and at most 80 characters. The name must consist of lowercase letters, numbers, hyphens and underscores, and it must be prefixed with a hash symbol.", channel)
}
