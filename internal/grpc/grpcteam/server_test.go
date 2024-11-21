//go:build integration_test

package grpcteam_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/grpc/grpcteam"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTeamsServer_Get(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("team not found", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		resp, err := grpcteam.NewServer(pool).Get(ctx, &protoapi.GetTeamRequest{Slug: "team-not-found"})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
			t.Errorf("expected status code %v, got %v", codes.NotFound, err)
		}
	})

	t.Run("get team", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		teamSlug := "team"
		purpose := "purpose"
		slackChannel := "#channel"
		entraIDgroupID := uuid.New()
		gitHubTeamSlug := "github-team-slug"
		googleGroupEmail := "mail@example.com"
		garRepository := "gar-repository"

		stmt := `
			INSERT INTO teams (slug, purpose, slack_channel, azure_group_id, github_team_slug, google_group_email, gar_repository) VALUES 
			($1, $2, $3, $4, $5, $6, $7)`
		if _, err = pool.Exec(ctx, stmt, teamSlug, purpose, slackChannel, entraIDgroupID, gitHubTeamSlug, googleGroupEmail, garRepository); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).Get(ctx, &protoapi.GetTeamRequest{Slug: teamSlug})
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

		if *resp.Team.EntraIdGroupId != entraIDgroupID.String() {
			t.Errorf("expected Azure group ID to be %q, got %q", entraIDgroupID.String(), *resp.Team.EntraIdGroupId)
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
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("missing slug", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		resp, err := grpcteam.NewServer(pool).Delete(ctx, &protoapi.DeleteTeamRequest{})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		}
	})

	t.Run("delete team", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		teamSlug := "team-slug"

		stmt := "INSERT INTO teams (slug, purpose, slack_channel, delete_key_confirmed_at) VALUES ($1, 'some purpose', '#channel', NOW())"
		if _, err := pool.Exec(ctx, stmt, teamSlug); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).Delete(ctx, &protoapi.DeleteTeamRequest{Slug: teamSlug})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp == nil {
			t.Error("expected response to be non-nil")
		}

		count := 0
		stmt = "SELECT COUNT(*) FROM teams WHERE slug = $1"
		if err := pool.QueryRow(ctx, stmt, teamSlug).Scan(&count); err != nil {
			t.Fatalf("failed to count teams: %v", err)
		} else if count != 0 {
			t.Fatalf("expected team to be deleted")
		}
	})
}

func TestTeamsServer_ToBeReconciled(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("fetch teams", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		stmt := "INSERT INTO teams (slug, purpose, slack_channel) VALUES ('team-1', 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		stmt = "INSERT INTO teams (slug, purpose, slack_channel) VALUES ('team-2', 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).List(ctx, &protoapi.ListTeamsRequest{
			Limit:  2,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 2 {
			t.Errorf("expected 2 teams, got %v", resp.Nodes)
		}

		if expected := "team-1"; resp.Nodes[0].Slug != expected {
			t.Errorf("expected first team to be %q, got %q", expected, resp.Nodes[0].Slug)
		}

		if expected := "team-2"; resp.Nodes[1].Slug != expected {
			t.Errorf("expected first team to be %q, got %q", expected, resp.Nodes[1].Slug)
		}
	})
}

func TestTeamsServer_IsRepositoryAuthorized(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("repo is authorized", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)

		stmt := "INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt, teamSlug); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		stmt = "INSERT INTO team_repositories (team_slug, github_repository) VALUES ($1, $2)"
		if _, err := pool.Exec(ctx, stmt, teamSlug, repoName); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:   teamSlug,
			Repository: repoName,
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
		pool := getConnection(ctx, t, container, dsn, log)

		const (
			teamSlug = "team-slug"
			repoName = "repo-name"
		)

		stmt := "INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt, teamSlug); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
			TeamSlug:   teamSlug,
			Repository: repoName,
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

func TestTeamsServer_Members(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("no members", func(t *testing.T) {
		teamSlug := slug.Slug("my-team")
		pool := getConnection(ctx, t, container, dsn, log)

		stmt := "INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt, teamSlug); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).Members(ctx, &protoapi.ListTeamMembersRequest{Slug: teamSlug.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 0 {
			t.Errorf("expected 0 members, got %v", len(resp.Nodes))
		}
	})

	t.Run("with members", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		teamSlug1 := slug.Slug("my-team")
		stmt := "INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt, teamSlug1); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		teamSlug2 := slug.Slug("other-team")
		stmt = "INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'some purpose', '#channel')"
		if _, err := pool.Exec(ctx, stmt, teamSlug2); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		userID1 := uuid.New()
		stmt = "INSERT INTO users (id, name, email, external_id) VALUES ($1, 'User 1', 'user1@example.com', '123')"
		if _, err = pool.Exec(ctx, stmt, userID1); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO user_roles (role_name, user_id, target_team_slug) VALUES ('Team member', $1, $2)"
		if _, err = pool.Exec(ctx, stmt, userID1, teamSlug1); err != nil {
			t.Fatalf("failed to insert user roles: %v", err)
		}

		userID2 := uuid.New()
		stmt = "INSERT INTO users (id, name, email, external_id) VALUES ($1, 'User 2', 'user2@example.com', '456')"
		if _, err = pool.Exec(ctx, stmt, userID2); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO user_roles (role_name, user_id, target_team_slug) VALUES ('Team owner', $1, $2)"
		if _, err = pool.Exec(ctx, stmt, userID2, teamSlug2); err != nil {
			t.Fatalf("failed to insert user roles: %v", err)
		}

		resp, err := grpcteam.NewServer(pool).Members(ctx, &protoapi.ListTeamMembersRequest{Slug: teamSlug1.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 1 {
			t.Errorf("expected 1 member, got %v", len(resp.Nodes))
		}

		member := resp.Nodes[0]
		if member.User.Name != "User 1" {
			t.Errorf("expected member name to be %q, got %q", "User 1", member.User.Name)
		}

		resp, err = grpcteam.NewServer(pool).Members(ctx, &protoapi.ListTeamMembersRequest{Slug: teamSlug2.String()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 1 {
			t.Errorf("expected 1 member, got %v", len(resp.Nodes))
		}

		member = resp.Nodes[0]
		if member.User.Name != "User 2" {
			t.Errorf("expected member name to be %q, got %q", "User 2", member.User.Name)
		}
	})
}

func startPostgresql(ctx context.Context, log logrus.FieldLogger) (container *postgres.PostgresContainer, dsn string, err error) {
	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

func getConnection(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) *pgxpool.Pool {
	pool, _ := database.NewPool(ctx, dsn, log, false)

	t.Cleanup(func() {
		pool.Close()
		if err := container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return pool
}
