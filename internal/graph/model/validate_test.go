package model_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

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
		if err := tpl.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	for _, s := range invalidChannels {
		tpl.SlackChannel = s
		if err := tpl.Validate(); err == nil {
			t.Errorf("expected error, but got nil")
		}
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
		if err := tpl.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	for _, s := range invalidSlugs {
		tpl.Slug = slug.Slug(s)
		if err := tpl.Validate(); err == nil {
			t.Errorf("expected error, but got nil")
		}
	}
}

func TestUpdateTeamInput_Validate(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("valid purpose"),
			SlackChannel: ptr.To("#valid-channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "prod",
					ChannelName: ptr.To("#name"),
				},
			},
		}

		if err := input.Validate([]string{"prod"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid purpose", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose: ptr.To(""),
		}

		if !errors.Is(input.Validate([]string{"prod"}), apierror.ErrTeamPurpose) {
			t.Fatalf("expected error %v, got %v", apierror.ErrTeamPurpose, input.Validate([]string{"prod"}))
		}
	})

	t.Run("invalid slack channel", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("purpose"),
			SlackChannel: ptr.To("#a"),
		}
		err := input.Validate([]string{"prod"})
		if contains := "The Slack channel does not fit the requirements"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %q", contains, err)
		}
	})

	t.Run("slack alerts channel with invalid environment", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("purpose"),
			SlackChannel: ptr.To("#channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "invalid",
					ChannelName: ptr.To("#channel"),
				},
			},
		}
		err := input.Validate([]string{"prod"})
		if contains := "The specified environment is not valid"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %q", contains, err)
		}
	})

	t.Run("slack alerts channel with invalid name", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("purpose"),
			SlackChannel: ptr.To("#channel"),
			SlackAlertsChannels: []*model.SlackAlertsChannelInput{
				{
					Environment: "prod",
					ChannelName: ptr.To("#a"),
				},
			},
		}
		err := input.Validate([]string{"prod"})
		if contains := "The Slack channel does not fit the requirements"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %q", contains, err)
		}
	})
}
