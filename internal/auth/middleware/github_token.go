package middleware

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/github/repository"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
)

// GitHubActorClaims holds the GitHub OIDC claims that are stored
// alongside activity log entries for audit purposes.
// See https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims.
type GitHubActorClaims struct {
	Actor                string `json:"actor"`
	ActorID              string `json:"actor_id"`
	BaseRef              string `json:"base_ref"`
	CheckRunID           string `json:"check_run_id"`
	Environment          string `json:"environment"`
	EventName            string `json:"event_name"`
	HeadRef              string `json:"head_ref"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSha       string `json:"job_workflow_sha"`
	Ref                  string `json:"ref"`
	RefType              string `json:"ref_type"`
	Repository           string `json:"repository"`
	RepositoryID         string `json:"repository_id"`
	RepositoryOwner      string `json:"repository_owner"`
	RepositoryOwnerID    string `json:"repository_owner_id"`
	RepositoryVisibility string `json:"repository_visibility"`
	RunAttempt           string `json:"run_attempt"`
	RunID                string `json:"run_id"`
	RunnerEnvironment    string `json:"runner_environment"`
	RunNumber            string `json:"run_number"`
	Workflow             string `json:"workflow"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSha          string `json:"workflow_sha"`
}

const (
	// GitHubOIDCIssuer is the OIDC issuer URL for GitHub Actions tokens.
	GitHubOIDCIssuer = "https://token.actions.githubusercontent.com"

	// GitHubOIDCAudience is the expected audience claim in GitHub OIDC tokens.
	GitHubOIDCAudience = "api.nais.io"
)

func GitHubOIDC(ctx context.Context, issuer string, log logrus.FieldLogger) (func(next http.Handler) http.Handler, error) {
	log = log.WithField("subsystem", "github_oidc")
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("initialize GitHub OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: GitHubOIDCAudience,
	})

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			token, ok := BearerAuth(r)
			if !ok {
				log.Debug("no valid bearer token found in request")
				next.ServeHTTP(w, r)
				return
			}

			idToken, err := verifier.Verify(r.Context(), token)
			if err != nil {
				log.WithError(err).Debug("failed to verify token")
				next.ServeHTTP(w, r)
				return
			}

			claims := GitHubActorClaims{}
			if err := idToken.Claims(&claims); err != nil {
				log.WithError(err).Debug("failed to parse claims from token")
				next.ServeHTTP(w, r)
				return
			}

			repos, err := repository.GetByName(ctx, claims.Repository)
			if err != nil {
				log.WithError(err).Debug("failed to get repository from token claims")
				next.ServeHTTP(w, r)
				return
			}

			if len(repos) == 0 {
				log.WithField("repository", claims.Repository).Debug("no repository found matching token claims")
				next.ServeHTTP(w, r)
				return
			}

			roles := []*authz.Role{}
			slugs := map[slug.Slug]struct{}{}
			for _, repo := range repos {
				repoRoles, err := authz.ForGitHubRepo(ctx, repo.TeamSlug)
				if err != nil {
					log.WithError(err).Debug("failed to get roles for github repo")
					next.ServeHTTP(w, r)
					return
				}
				roles = append(roles, repoRoles...)
				slugs[repo.TeamSlug] = struct{}{}
			}

			usr := &GitHubRepoActor{
				RepositoryName: claims.Repository,
				TeamSlugs:      slices.Collect(maps.Keys(slugs)),
				Claims:         claims,
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, usr, roles)))
		}
		return http.HandlerFunc(fn)
	}, nil
}

type GitHubRepoActor struct {
	RepositoryName string
	TeamSlugs      []slug.Slug
	Claims         GitHubActorClaims
}

func (g *GitHubRepoActor) GetID() uuid.UUID { return uuid.Nil }

func (g *GitHubRepoActor) Identity() string {
	return fmt.Sprintf("github-repo:%s", g.RepositoryName)
}

func (g *GitHubRepoActor) IsServiceAccount() bool { return true }

func (g *GitHubRepoActor) IsGitHubActions() {}

func (g *GitHubRepoActor) IsAdmin() bool { return false }

func (g *GitHubRepoActor) GCPTeamGroups(ctx context.Context) ([]string, error) {
	return team.ListGoogleGroupByTeamSlugs(ctx, g.TeamSlugs)
}
