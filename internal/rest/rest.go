package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/apply"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/rest/restteamsapi"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Fakes contains feature flags for local development and testing.
type Fakes struct {
	WithInsecureUserHeader bool
}

// Config holds all dependencies needed by the REST server.
type Config struct {
	ListenAddress        string
	Pool                 *pgxpool.Pool
	PreSharedKey         string
	ClusterConfigs       kubernetes.ClusterConfigMap
	DynamicClientFactory apply.DynamicClientFactory
	JWTMiddleware        func(http.Handler) http.Handler
	AuthHandler          authn.Handler
	Fakes                Fakes
	Log                  logrus.FieldLogger
}

func Run(ctx context.Context, cfg Config) error {
	router := MakeRouter(ctx, cfg)

	srv := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg.Log.Infof("REST HTTP server shutting down...")
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			cfg.Log.WithError(err).Infof("HTTP server shutdown failed")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		cfg.Log.Infof("REST HTTP server accepting requests on %q", cfg.ListenAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cfg.Log.WithError(err).Infof("unexpected error from HTTP server")
			return err
		}
		cfg.Log.Infof("REST HTTP server finished, terminating...")
		return nil
	})
	return wg.Wait()
}

func MakeRouter(ctx context.Context, cfg Config) *chi.Mux {
	router := chi.NewRouter()

	// Existing pre-shared-key authenticated routes.
	if cfg.PreSharedKey != "" {
		router.Group(func(r chi.Router) {
			r.Use(middleware.PreSharedKeyAuthentication(cfg.PreSharedKey))
			r.Get("/teams/{teamSlug}", restteamsapi.TeamsApiHandler(ctx, cfg.Pool, cfg.Log))
		})
	} else {
		// In test mode there is no pre-shared key; mount without auth.
		router.Group(func(r chi.Router) {
			r.Get("/teams/{teamSlug}", restteamsapi.TeamsApiHandler(ctx, cfg.Pool, cfg.Log))
		})
	}

	// Apply route with user authentication.
	if cfg.ClusterConfigs != nil {
		router.Group(func(r chi.Router) {
			// Context dependencies needed by authz and activitylog.
			r.Use(applyContextDependencies(cfg.Pool))

			if cfg.Fakes.WithInsecureUserHeader {
				r.Use(middleware.InsecureUserHeader())
			}

			if cfg.JWTMiddleware != nil {
				r.Use(cfg.JWTMiddleware)
			}

			r.Use(
				middleware.ApiKeyAuthentication(),
			)

			if cfg.AuthHandler != nil {
				r.Use(middleware.Oauth2Authentication(cfg.AuthHandler))
			}

			r.Use(
				middleware.GitHubOIDC(ctx, cfg.Log),
				middleware.RequireAuthenticatedUser(),
			)

			clientFactory := cfg.DynamicClientFactory
			if clientFactory == nil {
				clientFactory = apply.NewImpersonatingClientFactory(cfg.ClusterConfigs)
			}

			r.Post("/api/v1/apply", apply.Handler(cfg.ClusterConfigs, clientFactory, cfg.Log))
		})
	}

	return router
}

// applyContextDependencies returns a middleware that sets up the context with
// DB-backed loaders needed by the apply handler: database, authz, and activitylog.
// This is a minimal subset of what ConfigureGraph sets up — only what the apply
// handler actually needs.
func applyContextDependencies(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	setupContext := func(ctx context.Context) context.Context {
		ctx = database.NewLoaderContext(ctx, pool)
		ctx = user.NewLoaderContext(ctx, pool)
		ctx = serviceaccount.NewLoaderContext(ctx, pool)
		ctx = authz.NewLoaderContext(ctx, pool)
		ctx = activitylog.NewLoaderContext(ctx, pool)
		return ctx
	}
	return loader.Middleware(setupContext)
}
