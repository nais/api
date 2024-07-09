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
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graphv1/loaderv1"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/redis"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"github.com/vikstrous/dataloadgen"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// runHttpServer will start the HTTP server
func runHttpServer(ctx context.Context, listenAddress string, insecureAuth bool, db database.Database, k8sClient *k8s.Client, authHandler authn.Handler, graphHandler *handler.Server, graphv1Handler *handler.Server, reg prometheus.Gatherer, log logrus.FieldLogger) error {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})
	router.Method("GET", "/",
		otelhttp.WithRouteTag("playground", otelhttp.NewHandler(playground.Handler("GraphQL playground", "/query"), "playground")),
	)
	router.Method("GET", "/v1",
		otelhttp.WithRouteTag("playground", otelhttp.NewHandler(playground.Handler("GraphQL v1 playground", "/graphql"), "playground")),
	)

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
	)
	router.Route("/query", func(r chi.Router) {
		r.Use(middlewares...)
		r.Use(loader.Middleware(db))
		r.Use(otelhttp.NewMiddleware("graphql", otelhttp.WithPublicEndpoint(), otelhttp.WithSpanOptions(trace.WithAttributes(semconv.ServiceName("http")))))
		r.Method("POST", "/", otelhttp.WithRouteTag("query", graphHandler))
	})
	router.Route("/graphql", func(r chi.Router) {
		r.Use(middlewares...)
		r.Use(loaderv1.Middleware(func(ctx context.Context) context.Context {
			opts := []dataloadgen.Option{
				dataloadgen.WithWait(time.Millisecond),
				dataloadgen.WithBatchCapacity(250),
				dataloadgen.WithTracer(otel.Tracer("dataloader")),
			}

			pool := db.GetPool()
			ctx = application.NewLoaderContext(ctx, k8sClient, opts)
			ctx = bigquery.NewLoaderContext(ctx, k8sClient, opts)
			ctx = bucket.NewLoaderContext(ctx, k8sClient, opts)
			ctx = job.NewLoaderContext(ctx, k8sClient, opts)
			ctx = kafkatopic.NewLoaderContext(ctx, k8sClient, opts)
			ctx = opensearch.NewLoaderContext(ctx, k8sClient, opts)
			ctx = redis.NewLoaderContext(ctx, k8sClient, opts)
			ctx = sqlinstance.NewLoaderContext(ctx, k8sClient, opts)
			ctx = team.NewLoaderContext(ctx, pool, opts)
			ctx = user.NewLoaderContext(ctx, pool, opts)
			return ctx
		}))
		r.Use(otelhttp.NewMiddleware("graphqlv1", otelhttp.WithPublicEndpoint(), otelhttp.WithSpanOptions(trace.WithAttributes(semconv.ServiceName("http")))))
		r.Method("POST", "/", otelhttp.WithRouteTag("query", graphv1Handler))
	})

	router.Route("/oauth2", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/logout", authHandler.Logout)
		r.Get("/callback", authHandler.Callback)
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
