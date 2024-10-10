package api

import (
	"context"
	"errors"
	"fmt"
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
	legacysqlinstance "github.com/nais/api/internal/sqlinstance"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/cost"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/feedback"
	"github.com/nais/api/internal/v1/github/repository"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/persistence/bigquery"
	"github.com/nais/api/internal/v1/persistence/bucket"
	"github.com/nais/api/internal/v1/persistence/kafkatopic"
	"github.com/nais/api/internal/v1/persistence/opensearch"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/persistence/sqlinstance"
	"github.com/nais/api/internal/v1/reconciler"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/user"
	"github.com/nais/api/internal/v1/utilization"
	"github.com/nais/api/internal/v1/vulnerability"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/secret"
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
func runHttpServer(ctx context.Context, listenAddress string, insecureAuthAndFakes bool, tenantName string, clusters []string, db database.Database, watcherMgr *watcher.Manager, sqlAdminService *legacysqlinstance.SqlAdminService, authHandler authn.Handler, graphHandler *handler.Server, graphv1Handler *handler.Server, reg prometheus.Gatherer, vClient *vulnerability.Client, feedbackClient feedback.Client, log logrus.FieldLogger) error {
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

	middlewares = append(middlewares,
		cors.New(
			cors.Options{
				AllowedOrigins:   []string{"https://*", "http://*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowCredentials: true,
			},
		).Handler,
	)
	router.Route("/query", func(r chi.Router) {
		oldMiddlewares := append(middlewares,
			middleware.ApiKeyAuthentication(db),
			middleware.Oauth2Authentication(db, authHandler),
		)

		if insecureAuthAndFakes {
			oldMiddlewares = append([]func(http.Handler) http.Handler{auth.InsecureUserHeader(db)}, oldMiddlewares...)
		}
		r.Use(oldMiddlewares...)
		r.Use(loader.Middleware(db))
		r.Use(otelhttp.NewMiddleware("graphql", otelhttp.WithPublicEndpoint(), otelhttp.WithSpanOptions(trace.WithAttributes(semconv.ServiceName("http")))))
		r.Method("POST", "/", otelhttp.WithRouteTag("query", graphHandler))
	})

	graphMiddleware, err := ConfigureV1Graph(ctx, insecureAuthAndFakes, watcherMgr, db, sqlAdminService, vClient, tenantName, clusters, feedbackClient, log)
	if err != nil {
		return err
	}

	router.Route("/graphql", func(r chi.Router) {
		v1Middlewares := append(middlewares,
			middleware.Oauth2AuthenticationV1(db, authHandler),
			middleware.RequireAuthenticatedUser(),
		)

		if insecureAuthAndFakes {
			v1Middlewares = append([]func(http.Handler) http.Handler{auth.InsecureUserHeaderV1(db)}, v1Middlewares...)
		}
		r.Use(graphMiddleware)
		r.Use(v1Middlewares...)
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

func ConfigureV1Graph(ctx context.Context, fakeClients bool, watcherMgr *watcher.Manager, db database.Database, sqlAdminService *legacysqlinstance.SqlAdminService, vClient *vulnerability.Client, tenantName string, clusters []string, feedbackClient feedback.Client, log logrus.FieldLogger) (func(http.Handler) http.Handler, error) {
	appWatcher := application.NewWatcher(ctx, watcherMgr)
	jobWatcher := job.NewWatcher(ctx, watcherMgr)
	runWatcher := job.NewRunWatcher(ctx, watcherMgr)
	bqWatcher := bigquery.NewWatcher(ctx, watcherMgr)
	redisWatcher := redis.NewWatcher(ctx, watcherMgr)
	openSearchWatcher := opensearch.NewWatcher(ctx, watcherMgr)
	bucketWatcher := bucket.NewWatcher(ctx, watcherMgr)
	sqlDatabaseWatcher := sqlinstance.NewDatabaseWatcher(ctx, watcherMgr)
	sqlInstanceWatcher := sqlinstance.NewInstanceWatcher(ctx, watcherMgr)
	kafkaTopicWatcher := kafkatopic.NewWatcher(ctx, watcherMgr)
	secretWatcher := secret.NewWatcher(ctx, watcherMgr)

	var utilizationClient utilization.ResourceUsageClient
	if fakeClients {
		utilizationClient = utilization.NewFakeClient(clusters, nil, nil)
	} else {
		var err error
		utilizationClient, err = utilization.NewClient(clusters, tenantName, log)
		if err != nil {
			return nil, fmt.Errorf("create utilization client: %w", err)
		}
	}

	syncCtx, cancelSync := context.WithTimeout(ctx, 20*time.Second)
	defer cancelSync()
	if !watcherMgr.WaitForReady(syncCtx) {
		return nil, errors.New("timed out waiting for watchers to be ready")
	}

	dataloaderOpts := []dataloadgen.Option{
		dataloadgen.WithWait(time.Millisecond),
		dataloadgen.WithBatchCapacity(250),
		dataloadgen.WithTracer(otel.Tracer("dataloader")),
	}
	return loaderv1.Middleware(func(ctx context.Context) context.Context {
		ctx = application.NewLoaderContext(ctx, appWatcher)
		ctx = bigquery.NewLoaderContext(ctx, bqWatcher, dataloaderOpts)
		ctx = bucket.NewLoaderContext(ctx, bucketWatcher)
		ctx = job.NewLoaderContext(ctx, jobWatcher, runWatcher)
		ctx = kafkatopic.NewLoaderContext(ctx, kafkaTopicWatcher)
		ctx = secret.NewLoaderContext(ctx, secretWatcher)
		ctx = opensearch.NewLoaderContext(ctx, openSearchWatcher)
		ctx = redis.NewLoaderContext(ctx, redisWatcher)
		ctx = utilization.NewLoaderContext(ctx, utilizationClient)
		ctx = sqlinstance.NewLoaderContext(ctx, sqlAdminService, sqlDatabaseWatcher, sqlInstanceWatcher)
		ctx = feedback.NewLoaderContext(ctx, feedbackClient)
		pool := db.GetPool()
		ctx = databasev1.NewLoaderContext(ctx, pool)
		ctx = team.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = user.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = cost.NewLoaderContext(ctx, pool)
		ctx = repository.NewLoaderContext(ctx, pool)
		ctx = role.NewLoaderContext(ctx, pool)
		ctx = auditv1.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = vulnerability.NewLoaderContext(ctx, vClient, log, dataloaderOpts)
		ctx = reconciler.NewLoaderContext(ctx, pool, dataloaderOpts)
		return ctx
	}), nil
}
