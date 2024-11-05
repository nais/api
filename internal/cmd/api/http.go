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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/cost"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/deployment"
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
	"github.com/nais/api/internal/v1/serviceaccount"
	"github.com/nais/api/internal/v1/session"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/unleash"
	"github.com/nais/api/internal/v1/user"
	"github.com/nais/api/internal/v1/utilization"
	"github.com/nais/api/internal/v1/vulnerability"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/podlog"
	fakepodlog "github.com/nais/api/internal/v1/workload/podlog/fake"
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
	"k8s.io/client-go/kubernetes"
)

// runHttpServer will start the HTTP server
func runHttpServer(
	ctx context.Context,
	listenAddress string,
	insecureAuthAndFakes bool,
	tenantName string,
	clusters []string,
	db database.Database,
	k8sClientSets map[string]kubernetes.Interface,
	watcherMgr *watcher.Manager,
	mgmtWatcherMgr *watcher.Manager,
	authHandler authn.Handler,
	graphHandler *handler.Server,
	reg prometheus.Gatherer,
	vClient vulnerability.Client,
	hookdClient hookd.Client,
	log logrus.FieldLogger,
) error {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})
	router.Method("GET", "/",
		otelhttp.WithRouteTag("playground", otelhttp.NewHandler(playground.Handler("GraphQL playground", "/graphql"), "playground")),
	)

	graphMiddleware, err := ConfigureGraph(ctx, insecureAuthAndFakes, watcherMgr, mgmtWatcherMgr, db.GetPool(), k8sClientSets, vClient, tenantName, clusters, hookdClient, log)
	if err != nil {
		return err
	}

	router.Route("/graphql", func(r chi.Router) {
		middlewares := []func(http.Handler) http.Handler{
			graphMiddleware,
			cors.New(
				cors.Options{
					AllowedOrigins:   []string{"https://*", "http://*"},
					AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
					AllowCredentials: true,
				},
			).Handler,
		}

		if insecureAuthAndFakes {
			middlewares = append(middlewares, middleware.InsecureUserHeader())
		}

		middlewares = append(
			middlewares,
			middleware.ApiKeyAuthentication(),
			middleware.Oauth2Authentication(authHandler),
			middleware.RequireAuthenticatedUser(),
			otelhttp.NewMiddleware(
				"graphql",
				otelhttp.WithPublicEndpoint(),
				otelhttp.WithSpanOptions(trace.WithAttributes(semconv.ServiceName("http"))),
			),
		)
		r.Use(middlewares...)
		r.Method("POST", "/", otelhttp.WithRouteTag("query", graphHandler))
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

func ConfigureGraph(
	ctx context.Context,
	fakeClients bool,
	watcherMgr *watcher.Manager,
	mgmtWatcherMgr *watcher.Manager,
	pool *pgxpool.Pool,
	k8sClientSets map[string]kubernetes.Interface,
	vClient vulnerability.Client,
	tenantName string,
	clusters []string,
	hookdClient hookd.Client,
	log logrus.FieldLogger,
) (func(http.Handler) http.Handler, error) {
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
	podWatcher := workload.NewWatcher(ctx, watcherMgr)
	ingressWatcher := application.NewIngressWatcher(ctx, watcherMgr)
	unleashWatcher := unleash.NewWatcher(ctx, mgmtWatcherMgr)

	sqlAdminService, err := sqlinstance.NewClient(ctx, log, sqlinstance.WithFakeClients(fakeClients), sqlinstance.WithInstanceWatcher(sqlInstanceWatcher))
	if err != nil {
		return nil, fmt.Errorf("create SQL Admin service: %w", err)
	}

	var utilizationClient utilization.ResourceUsageClient
	var costOpts []cost.Option
	var podLogStreamer podlog.Streamer
	if fakeClients {
		utilizationClient = utilization.NewFakeClient(clusters, nil, nil)
		costOpts = append(costOpts, cost.WithClient(cost.NewFakeClient()))
		podLogStreamer = fakepodlog.NewLogStreamer()
	} else {
		var err error
		utilizationClient, err = utilization.NewClient(clusters, tenantName, log)
		if err != nil {
			return nil, fmt.Errorf("create utilization client: %w", err)
		}
		podLogStreamer = podlog.NewLogStreamer(k8sClientSets, log)
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
		ctx = podlog.NewLoaderContext(ctx, podLogStreamer)
		ctx = application.NewLoaderContext(ctx, appWatcher, ingressWatcher)
		ctx = bigquery.NewLoaderContext(ctx, bqWatcher, dataloaderOpts)
		ctx = bucket.NewLoaderContext(ctx, bucketWatcher)
		ctx = job.NewLoaderContext(ctx, jobWatcher, runWatcher)
		ctx = kafkatopic.NewLoaderContext(ctx, kafkaTopicWatcher)
		ctx = workload.NewLoaderContext(ctx, podWatcher)
		ctx = secret.NewLoaderContext(ctx, secretWatcher, log)
		ctx = opensearch.NewLoaderContext(ctx, openSearchWatcher)
		ctx = redis.NewLoaderContext(ctx, redisWatcher)
		ctx = utilization.NewLoaderContext(ctx, utilizationClient)
		ctx = sqlinstance.NewLoaderContext(ctx, sqlAdminService, sqlDatabaseWatcher, sqlInstanceWatcher, dataloaderOpts)
		ctx = databasev1.NewLoaderContext(ctx, pool)
		ctx = team.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = user.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = cost.NewLoaderContext(ctx, pool, costOpts...)
		ctx = repository.NewLoaderContext(ctx, pool)
		ctx = role.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = auditv1.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = vulnerability.NewLoaderContext(ctx, vClient, tenantName, clusters, fakeClients, log, dataloaderOpts)
		ctx = reconciler.NewLoaderContext(ctx, pool, dataloaderOpts)
		ctx = deployment.NewLoaderContext(ctx, hookdClient)
		ctx = serviceaccount.NewLoaderContext(ctx, pool)
		ctx = session.NewLoaderContext(ctx, pool)
		ctx = unleash.NewLoaderContext(ctx, tenantName, unleashWatcher, "*fake*", log)
		return ctx
	}), nil
}
