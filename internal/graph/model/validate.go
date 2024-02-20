package model

import (
	"regexp"
	"strings"

	"github.com/nais/api/internal/graph/apierror"
)

var (
	// Slightly modified from database schema because Golang doesn't like Perl-flavored regexes.
	teamSlugRegex = regexp.MustCompile("^[a-z](-?[a-z0-9]+)+$")

	// Rules can be found here: https://api.slack.com/methods/conversations.create#naming
	slackChannelNameRegex = regexp.MustCompile("^#[a-z0-9æøå_-]{2,80}$")

	// Slugs that are reserved
	reservedSlugs = []string{
		"nais-system",
		"kube-system",
		"kube-node-lease",
		"kube-public",
		"kyverno",
		"cnrm-system",
		"configconnector-operator-system",
	}
)

func (input CreateTeamInput) Validate() error {
	if !teamSlugRegex.MatchString(string(input.Slug)) || len(input.Slug) < 3 || len(input.Slug) > 30 {
		return apierror.ErrTeamSlug
	}

	if input.Purpose == "" {
		return apierror.ErrTeamPurpose
	}

	slug := input.Slug.String()

	if strings.HasPrefix(slug, "team") {
		return apierror.ErrTeamPrefixRedundant
	}

	for _, reserved := range reservedSlugs {
		if slug == reserved {
			return apierror.ErrTeamSlugReserved
		}
	}

	if !slackChannelNameRegex.MatchString(input.SlackChannel) {
		return slackChannelError(input.SlackChannel)
	}

	return nil
}

func (input UpdateTeamInput) Validate(validEnvironments []string) error {
	if input.Purpose != nil && *input.Purpose == "" {
		return apierror.ErrTeamPurpose
	}

	if input.SlackChannel != nil && !slackChannelNameRegex.MatchString(*input.SlackChannel) {
		return slackChannelError(*input.SlackChannel)
	}

	validEnvironment := func(env string) bool {
		for _, environment := range validEnvironments {
			if env == environment {
				return true
			}
		}
		return false
	}

	for _, entry := range input.SlackAlertsChannels {
		if !validEnvironment(entry.Environment) {
			return apierror.Errorf("The specified environment is not valid: %q. Valid environments are: %s.", entry.Environment, strings.Join(validEnvironments, ", "))
		}

		if entry.ChannelName != nil && !slackChannelNameRegex.MatchString(*entry.ChannelName) {
			return slackChannelError(*entry.ChannelName)
		}
	}

	return nil
}

func slackChannelError(channel string) apierror.Error {
	return apierror.Errorf("The Slack channel does not fit the requirements: %q. The name must contain at least 2 characters and at most 80 characters. The name must consist of lowercase letters, numbers, hyphens and underscores, and it must be prefixed with a hash symbol.", channel)
}
