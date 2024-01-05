package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/apierror"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](value T) *T {
	return &value
}

func TestCreateTeamInput_Validate_SlackChannel(t *testing.T) {
	tpl := model.CreateTeamInput{
		Slug:    slug.Slug("valid-slug"),
		Purpose: "valid purpose",
	}

	validChannels := []string{
		"#foo",
		"#foo-bar",
		"#æøå",
		"#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	invalidChannels := []string{
		"foo", // missing hash
		"#a",  // too short
		"#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", // too long
		"#foo bar", // space not allowed
		"#Foobar",  // upper case not allowed
	}

	for _, s := range validChannels {
		tpl.SlackChannel = s
		assert.NoError(t, tpl.Validate(), "Slack channel %q should pass validation, but didn't", tpl.SlackChannel)
	}

	for _, s := range invalidChannels {
		tpl.SlackChannel = s
		assert.Error(t, tpl.Validate(), "Slack channel %q passed validation even if it should not", tpl.SlackChannel)
	}
}

func TestCreateTeamInput_Validate_Slug(t *testing.T) {
	tpl := model.CreateTeamInput{
		Slug:         "",
		Purpose:      "valid purpose",
		SlackChannel: "#channel",
	}

	validSlugs := []string{
		"foo",
		"foo-bar",
		"f00b4r",
		"channel4",
		"some-long-string-less-than-31c",
		"nais",
		"naisuratur",
		"naisan",
	}

	invalidSlugs := []string{
		"a",
		"ab",
		"-foo",
		"foo-",
		"foo--bar",
		"4chan",
		"team",
		"team-foo",
		"teamfoobar",
		"some-long-string-more-than-30-chars",
		"you-aint-got-the-æøå",
		"Uppercase",
		"rollback()",
		"kube-node-lease",
		"kube-public",
		"kube-system",
		"nais-system",
		"kyverno",
		"cnrm-system",
		"configconnector-operator-system",
	}

	for _, s := range validSlugs {
		tpl.Slug = slug.Slug(s)
		assert.NoError(t, tpl.Validate(), "Slug %q should pass validation, but didn't", tpl.Slug)
	}

	for _, s := range invalidSlugs {
		tpl.Slug = slug.Slug(s)
		assert.Error(t, tpl.Validate(), "Slug %q passed validation even if it should not", tpl.Slug)
	}
}

func TestUpdateTeamInput_Validate(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr("valid purpose"),
			SlackChannel: ptr("#valid-channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "prod",
					ChannelName: ptr("#name"),
				},
			},
		}
		assert.Nil(t, input.Validate([]string{"prod"}))
	})

	t.Run("invalid purpose", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose: ptr(""),
		}
		assert.Error(t, apierror.ErrTeamPurpose, input.Validate([]string{"prod"}))
	})

	t.Run("invalid slack channel", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr("purpose"),
			SlackChannel: ptr("#a"),
		}
		assert.ErrorContains(t, input.Validate([]string{"prod"}), "The Slack channel does not fit the requirements")
	})

	t.Run("slack alerts channel with invalid environment", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr("purpose"),
			SlackChannel: ptr("#channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "invalid",
					ChannelName: ptr("#channel"),
				},
			},
		}
		assert.ErrorContains(t, input.Validate([]string{"prod"}), "The specified environment is not valid")
	})

	t.Run("slack alerts channel with invalid name", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr("purpose"),
			SlackChannel: ptr("#channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "prod",
					ChannelName: ptr("#a"),
				},
			},
		}
		assert.ErrorContains(t, input.Validate([]string{"prod"}), "The Slack channel does not fit the requirements")
	})
}
