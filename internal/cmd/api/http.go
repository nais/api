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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// runHttpServer will start the HTTP server
func runHttpServer(
	ctx context.Context,
	listenAddress string,
	insecureAuth bool,
	db database.Database,
	authHandler authn.Handler,
	graphHandler *handler.Server,
	reg prometheus.Gatherer,
	log logrus.FieldLogger,
) error {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})
	router.Method("GET", "/",
		otelhttp.WithRouteTag("playground", otelhttp.NewHandler(playground.Handler("GraphQL playground", "/query"), "playground")),
	)

	dataLoaders := dataloader.NewLoaders(db)
	middlewares := []func(http.Handler) http.Handler{}

	if insecureAuth {
		middlewares = append(middlewares, auth.InsecureUserHeader(db))
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
		r.Use(otelhttp.NewMiddleware("graphql", otelhttp.WithPublicEndpoint(), otelhttp.WithSpanOptions(trace.WithAttributes(semconv.ServiceName("http")))))
		r.Method("POST", "/", otelhttp.WithRouteTag("query", graphHandler))
	})

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: router,
	}

	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Infof("HTTP server shutting down...")
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Infof("HTTP server shutdown failed")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		log.Infof("HTTP server accepting requests on %q", listenAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Infof("unexpected error from HTTP server")
			return err
		}
		log.Infof("HTTP server finished, terminating...")
		return nil
	})
	return wg.Wait()
}
