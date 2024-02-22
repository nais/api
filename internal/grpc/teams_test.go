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
	t.Run("team not found", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamBySlug(ctx, slug.Slug("team-not-found")).
			Return(nil, pgx.ErrNoRows).
			Once()

		resp, err := grpc.NewTeamsServer(db).Get(ctx, &protoapi.GetTeamRequest{Slug: "team-not-found"})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
			t.Errorf("expected status code %v, got %v", codes.NotFound, err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamBySlug(ctx, slug.Slug("team-not-found")).
			Return(nil, fmt.Errorf("some database error")).
			Once()

		resp, err := grpc.NewTeamsServer(db).Get(ctx, &protoapi.GetTeamRequest{Slug: "team-not-found"})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
			t.Errorf("expected status code %v, got %v", codes.Internal, err)
		}
	})

	t.Run("get team", func(t *testing.T) {
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
			t.Error("expected response to be non-nil")
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

func TestTeamsServer_Delete(t *testing.T) {
	ctx := context.Background()
	t.Run("missing slug", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		resp, err := grpc.NewTeamsServer(db).Delete(ctx, &protoapi.DeleteTeamRequest{})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		const teamSlug = "team-slug"
		db := database.NewMockDatabase(t)
		db.EXPECT().
			DeleteTeam(ctx, slug.Slug(teamSlug)).
			Return(nil).
			Once()
		resp, err := grpc.NewTeamsServer(db).Delete(ctx, &protoapi.DeleteTeamRequest{Slug: teamSlug})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp == nil {
			t.Error("expected response to be non-nil")
		}
	})
}

func TestTeamsServer_List(t *testing.T) {
	ctx := context.Background()
	t.Run("error when fetching teams from database", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeams(ctx, database.Page{Limit: 123, Offset: 2}).
			Return(nil, 0, fmt.Errorf("some error")).
			Once()
		resp, err := grpc.NewTeamsServer(db).List(ctx, &protoapi.ListTeamsRequest{
			Limit:  123,
			Offset: 2,
		})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
			t.Errorf("expected status code %v, got %v", codes.Internal, err)
		}
	})

	t.Run("fetch teams", func(t *testing.T) {
		teamsFromDatabase := []*database.Team{
			{Team: &gensql.Team{Slug: "team1"}},
			{Team: &gensql.Team{Slug: "team2"}},
		}
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeams(ctx, database.Page{Limit: 2, Offset: 0}).
			Return(teamsFromDatabase, 2, nil).
			Once()
		resp, err := grpc.NewTeamsServer(db).List(ctx, &protoapi.ListTeamsRequest{
			Limit:  2,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 2 {
			t.Errorf("expected 2 teams, got %v", resp.Nodes)
		}

		if expected := "team1"; resp.Nodes[0].Slug != expected {
			t.Errorf("expected first team to be %q, got %q", expected, resp.Nodes[0].Slug)
		}

		if expected := "team2"; resp.Nodes[1].Slug != expected {
			t.Errorf("expected first team to be %q, got %q", expected, resp.Nodes[1].Slug)
		}
	})
}

func TestTeamsServer_IsRepositoryAuthorized(t *testing.T) {
	ctx := context.Background()
	t.Run("error when fetching authorizations from database", func(t *testing.T) {
		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetRepositoryAuthorizations(ctx, slug.Slug(teamSlug), repoName).
			Return(nil, fmt.Errorf("some error")).
			Once()
		resp, err := grpc.NewTeamsServer(db).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:   teamSlug,
			Repository: repoName,
		})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
			t.Errorf("expected status code %v, got %v", codes.Internal, err)
		}
	})

	t.Run("invalid authorization", func(t *testing.T) {
		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetRepositoryAuthorizations(ctx, slug.Slug(teamSlug), repoName).
			Return([]gensql.RepositoryAuthorizationEnum{}, nil).
			Once()
		resp, err := grpc.NewTeamsServer(db).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:      teamSlug,
			Repository:    repoName,
			Authorization: protoapi.RepositoryAuthorization_UNKNOWN,
		})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		}
	})

	t.Run("repo is authorized", func(t *testing.T) {
		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetRepositoryAuthorizations(ctx, slug.Slug(teamSlug), repoName).
			Return([]gensql.RepositoryAuthorizationEnum{
				gensql.RepositoryAuthorizationEnumDeploy,
			}, nil).
			Once()
		resp, err := grpc.NewTeamsServer(db).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:      teamSlug,
			Repository:    repoName,
			Authorization: protoapi.RepositoryAuthorization_DEPLOY,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp == nil {
			t.Fatalf("expected response to be non nil")
		}

		if !resp.IsAuthorized {
			t.Errorf("expected repository to be authorized")
		}
	})

	t.Run("repo is not authorized", func(t *testing.T) {
		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)
		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetRepositoryAuthorizations(ctx, slug.Slug(teamSlug), repoName).
			Return([]gensql.RepositoryAuthorizationEnum{}, nil).
			Once()
		resp, err := grpc.NewTeamsServer(db).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:      teamSlug,
			Repository:    repoName,
			Authorization: protoapi.RepositoryAuthorization_DEPLOY,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp == nil {
			t.Fatalf("expected response to be non nil")
		}

		if resp.IsAuthorized {
			t.Errorf("did not expect repository to be authorized")
		}
	})
}
