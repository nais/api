package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/nais/api/internal/auth"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/dataloader"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// runHttpServer will start the HTTP server
func runHttpServer(
	ctx context.Context,
	cancel context.CancelFunc,
	cfg *Config,
	db database.Database,
	authHandler authn.Handler,
	graphHandler *handler.Server,
	log logrus.FieldLogger,
) {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})
	router.Get("/", playground.Handler("GraphQL playground", "/query"))

	dataLoaders := dataloader.NewLoaders(db)
	middlewares := []func(http.Handler) http.Handler{}

	if cfg.WithFakeClients {
		middlewares = append(middlewares, auth.StaticUser(db))
	}

	middlewares = append(middlewares,
		cors.New(
			cors.Options{
				AllowedOrigins:   []string{"https://*", "http://*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowCredentials: true,
			},
		).Handler,

		middleware.ApiKeyAuthentication(db),
		middleware.Oauth2Authentication(db, authHandler),
		dataloader.Middleware(dataLoaders),
	)
	router.Route("/query", func(r chi.Router) {
		r.Use(middlewares...)
		r.Post("/", graphHandler.ServeHTTP)
	})

	srv := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: router,
	}
	defer cancel()

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Infof("HTTP server shutting down...")
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Infof("HTTP server shutdown failed")
		}
	}()

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.WithError(err).Infof("unexpected error from HTTP server")
	}
	log.Infof("HTTP server finished, terminating...")
}
