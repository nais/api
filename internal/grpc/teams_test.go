package grpc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/grpc"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

func TestTeamsServer_Get(t *testing.T) {
	ctx := context.Background()
	t.Run("team not found", func(t1 *testing.T) {
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamBySlug(ctx, slug.Slug("team-not-found")).
			Return(nil, pgx.ErrNoRows).
			Once()

		resp, err := grpc.NewTeamsServer(db).Get(ctx, &protoapi.GetTeamRequest{Slug: "team-not-found"})
		if resp != nil {
			t.Error("expected team to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
			t.Errorf("expected status.NotFound, got %v", err)
		}
	})

	t.Run("database error", func(t1 *testing.T) {
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamBySlug(ctx, slug.Slug("team-not-found")).
			Return(nil, fmt.Errorf("some database error")).
			Once()

		resp, err := grpc.NewTeamsServer(db).Get(ctx, &protoapi.GetTeamRequest{Slug: "team-not-found"})
		if resp != nil {
			t.Error("expected team to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
			t.Errorf("expected status.NotFound, got %v", err)
		}
	})

	t.Run("get team", func(t1 *testing.T) {
		const (
			teamSlug         = "team"
			purpose          = "purpose"
			slackChannel     = "slackChannel"
			gitHubTeamSlug   = "github-team-slug"
			googleGroupEmail = "mail@example.com"
			garRepository    = "gar-repository"
		)

		aid := uuid.New()

		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamBySlug(ctx, slug.Slug(teamSlug)).
			Return(&database.Team{Team: &gensql.Team{
				Slug:             teamSlug,
				Purpose:          purpose,
				SlackChannel:     slackChannel,
				AzureGroupID:     &aid,
				GithubTeamSlug:   ptr.To(gitHubTeamSlug),
				GoogleGroupEmail: ptr.To(googleGroupEmail),
				GarRepository:    ptr.To(garRepository),
			}}, nil).
			Once()

		resp, err := grpc.NewTeamsServer(db).Get(ctx, &protoapi.GetTeamRequest{Slug: teamSlug})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Team == nil {
			t.Error("expected team to be non-nil")
		}

		if resp.Team.Slug != teamSlug {
			t.Errorf("expected team slug to be %q, got %q", teamSlug, resp.Team.Slug)
		}

		if resp.Team.Purpose != purpose {
			t.Errorf("expected team purpose to be %q, got %q", purpose, resp.Team.Purpose)
		}

		if resp.Team.SlackChannel != slackChannel {
			t.Errorf("expected Slack channel to be %q, got %q", slackChannel, resp.Team.SlackChannel)
		}

		if *resp.Team.AzureGroupId != aid.String() {
			t.Errorf("expected Azure group ID to be %q, got %q", aid.String(), *resp.Team.AzureGroupId)
		}

		if *resp.Team.GithubTeamSlug != gitHubTeamSlug {
			t.Errorf("expected GitHub team slug to be %q, got %q", gitHubTeamSlug, *resp.Team.GithubTeamSlug)
		}

		if *resp.Team.GoogleGroupEmail != googleGroupEmail {
			t.Errorf("expected Google group email to be %q, got %q", googleGroupEmail, *resp.Team.GoogleGroupEmail)
		}

		if *resp.Team.GarRepository != garRepository {
			t.Errorf("expected GAR repository to be %q, got %q", garRepository, *resp.Team.GarRepository)
		}
	})
}
