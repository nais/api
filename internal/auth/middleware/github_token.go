package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/github/repository"
	"github.com/sirupsen/logrus"
)

// ghClaims represents the claims present in a GitHub OIDC token
// See https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims.
type ghClaims struct {
	// Ref               string `json:"ref"`
	Repository string `json:"repository"`
	// RepositoryID      string `json:"repository_id"`
	// RepositoryOwner   string `json:"repository_owner"`
	// RepositoryOwnerID string `json:"repository_owner_id"`
	// RunID             string `json:"run_id"`
	// RunNumber         string `json:"run_number"`
	// RunAttempt        string `json:"run_attempt"`
	// Actor             string `json:"actor"`
	// ActorID           string `json:"actor_id"`
	// Workflow          string `json:"workflow"`
	// HeadRef           string `json:"head_ref"`
	// BaseRef           string `json:"base_ref"`
	// EventName         string `json:"event_name"`
	// RefType           string `json:"ref_type"`
	// Environment       string `json:"environment"`
	// JobWorkflowRef    string `json:"job_workflow_ref"`
}

func GitHubOIDC(ctx context.Context, log logrus.FieldLogger) func(next http.Handler) http.Handler {
	log = log.WithField("subsystem", "github_oidc")
	provider, err := oidc.NewProvider(ctx, "https://token.actions.githubusercontent.com")
	if err != nil {
		log.WithError(err).Error("failed to initialize GitHub OIDC provider. Will not support GitHub OIDC authentication")
		return func(sub http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sub.ServeHTTP(w, r)
			})
		}
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: "api.nais.io",
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

			claims := &ghClaims{}
			if err := idToken.Claims(claims); err != nil {
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
			for _, repo := range repos {
				repoRoles, err := authz.ForGitHubRepo(ctx, repo.TeamSlug)
				if err != nil {
					log.WithError(err).Debug("failed to get roles for github repo")
					next.ServeHTTP(w, r)
					return
				}
				roles = append(roles, repoRoles...)
			}

			usr := &GitHubRepoActor{
				RepositoryName: claims.Repository,
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, usr, roles)))
		}
		return http.HandlerFunc(fn)
	}
}

type GitHubRepoActor struct {
	RepositoryName string
}

func (g *GitHubRepoActor) GetID() uuid.UUID { return uuid.Nil }

func (g *GitHubRepoActor) Identity() string {
	return fmt.Sprintf("github-repo:%s", g.RepositoryName)
}

func (g *GitHubRepoActor) IsServiceAccount() bool { return true }

func (g *GitHubRepoActor) IsAdmin() bool { return false }
