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
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/deployment"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/feature"
	"github.com/nais/api/internal/github/repository"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/issue"
	apik8s "github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/loki"
	"github.com/nais/api/internal/metrics"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/price"
	fakeprice "github.com/nais/api/internal/price/fake"
	"github.com/nais/api/internal/reconciler"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/servicemaintenance"
	"github.com/nais/api/internal/session"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/thirdparty/promclient"
	promfake "github.com/nais/api/internal/thirdparty/promclient/fake"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync"
	"github.com/nais/api/internal/utilization"
	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/logging"
	"github.com/nais/api/internal/workload/podlog"
	fakepodlog "github.com/nais/api/internal/workload/podlog/fake"
	"github.com/nais/api/internal/workload/secret"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// runHttpServer will start the HTTP server
func runHttpServer(
	ctx context.Context,
	fakes Fakes,
	listenAddress string,
	tenantName string,
	clusters []string,
	pool *pgxpool.Pool,
	k8sClients apik8s.ClusterConfigMap,
	watchers *watchers.Watchers,
	watcherMgr *watcher.Manager,
	jwtMiddleware func(http.Handler) http.Handler,
	authHandler authn.Handler,
	graphHandler *handler.Server,
	serviceMaintenanceManager *servicemaintenance.Manager,
	aivenClient aiven.AivenClient,
	aivenProjects aiven.Projects,
	vulnMgr *vulnerability.Manager,
	hookdClient hookd.Client,
	bifrostAPIURL string,
	allowedClusters []string,
	defaultLogDestinations []logging.SupportedLogDestination,
	notifier *notify.Notifier,
	lokiClient loki.Client,
	auditLogProjectID string,
	auditLogLocation string,
	log logrus.FieldLogger,
) error {
	router := chi.NewRouter()
	router.Method("GET", "/",
		otelhttp.WithRouteTag("playground", otelhttp.NewHandler(playground.Handler("GraphQL playground", "/graphql"), "playground")),
	)

	graphMiddleware, err := ConfigureGraph(
		ctx,
		fakes,
		watchers,
		watcherMgr,
		pool,
		k8sClients,
		serviceMaintenanceManager,
		aivenClient,
		aivenProjects,
		vulnMgr,
		tenantName,
		clusters,
		hookdClient,
		bifrostAPIURL,
		allowedClusters,
		defaultLogDestinations,
		notifier,
		lokiClient,
		auditLogProjectID,
		auditLogLocation,
		log,
	)
	if err != nil {
		return err
	}

	contextDependencies := graphMiddleware

	router.Route("/graphql", func(r chi.Router) {
		middlewares := []func(http.Handler) http.Handler{
			contextDependencies,
			cors.New(
				cors.Options{
					AllowedOrigins:   []string{"https://*", "http://*"},
					AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
					AllowCredentials: true,
				},
			).Handler,
		}

		if fakes.WithInsecureUserHeader {
			middlewares = append(middlewares, middleware.InsecureUserHeader())
		}

		if jwtMiddleware != nil {
			middlewares = append(middlewares, jwtMiddleware)
		}

		middlewares = append(
			middlewares,
			middleware.ApiKeyAuthentication(),
			middleware.Oauth2Authentication(authHandler),
			middleware.GitHubOIDC(ctx, log),
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
		r.Use(contextDependencies)
		r.Get("/login", authHandler.Login)
		r.Get("/logout", authHandler.Logout)
		r.Get("/callback", authHandler.Callback)
	})

	srv := &http.Server{
		Addr:              listenAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
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
	fakes Fakes,
	watchers *watchers.Watchers,
	watcherMgr *watcher.Manager,
	pool *pgxpool.Pool,
	k8sClients apik8s.ClusterConfigMap,
	serviceMaintenanceManager *servicemaintenance.Manager,
	aivenClient aiven.AivenClient,
	aivenProjects aiven.Projects,
	vulnMgr *vulnerability.Manager,
	tenantName string,
	clusters []string,
	hookdClient hookd.Client,
	bifrostAPIURL string,
	allowedClusters []string,
	defaultLogDestinations []logging.SupportedLogDestination,
	notifier *notify.Notifier,
	lokiClient loki.Client,
	auditLogProjectID string,
	auditLogLocation string,
	log logrus.FieldLogger,
) (func(http.Handler) http.Handler, error) {
	searcher, err := search.New(ctx, pool, log.WithField("subsystem", "search_bleve"))
	if err != nil {
		return nil, fmt.Errorf("init bleve: %w", err)
	}

	// Searchers searchers
	application.AddSearch(searcher, watchers.AppWatcher)
	job.AddSearch(searcher, watchers.JobWatcher)
	bigquery.AddSearch(searcher, watchers.BqWatcher)
	bucket.AddSearch(searcher, watchers.BucketWatcher)
	kafkatopic.AddSearch(searcher, watchers.KafkaTopicWatcher)
	opensearch.AddSearch(searcher, watchers.OpenSearchWatcher)
	sqlinstance.AddSearch(searcher, watchers.SqlInstanceWatcher)
	valkey.AddSearch(searcher, watchers.ValkeyWatcher)
	team.AddSearch(searcher, pool, notifier, log.WithField("subsystem", "team_search"))

	// Re-index all to initialize the search index
	if err := searcher.ReIndex(ctx); err != nil {
		return nil, fmt.Errorf("reindex all: %w", err)
	}

	sqlAdminService, err := sqlinstance.NewClient(ctx, log, sqlinstance.WithFakeClients(fakes.WithFakeCloudSQL), sqlinstance.WithInstanceWatcher(watchers.SqlInstanceWatcher))
	if err != nil {
		return nil, fmt.Errorf("create SQL Admin service: %w", err)
	}

	var priceRetriever price.Retriever
	if fakes.WithFakePriceClient {
		priceRetriever = fakeprice.NewClient()
		log.Warn("Using fake price retriever")
	} else {
		priceRetriever, err = price.NewClient(ctx, log)
		if err != nil {
			return nil, fmt.Errorf("create price service: %w", err)
		}
	}

	var prometheusClient promclient.Client
	if fakes.WithFakePrometheus {
		prometheusClient = promfake.NewFakeClient(clusters, nil, nil)
	} else {
		var err error
		prometheusClient, err = promclient.New(clusters, tenantName, log)
		if err != nil {
			return nil, fmt.Errorf("create utilization client: %w", err)
		}
	}

	var podLogStreamer podlog.Streamer
	var secretClientCreator secret.ClientCreator
	if fakes.WithFakeKubernetes {
		podLogStreamer = fakepodlog.NewLogStreamer()
		secretClientCreator = secret.CreatorFromClients(watcherMgr.GetDynamicClients())
	} else {
		clients, err := apik8s.NewClientSets(k8sClients)
		if err != nil {
			return nil, fmt.Errorf("create k8s client sets: %w", err)
		}
		podLogStreamer = podlog.NewLogStreamer(clients, log)
		secretClientCreator = secret.CreatorFromConfig(ctx, k8sClients)
	}

	var costOpts []cost.Option
	if fakes.WithFakeCostClient {
		costOpts = append(costOpts, cost.WithClient(cost.NewFakeClient()))
	}

	syncCtx, cancelSync := context.WithTimeout(ctx, 20*time.Second)
	defer cancelSync()
	if !watcherMgr.WaitForReady(syncCtx) {
		return nil, errors.New("timed out waiting for watchers to be ready")
	}

	setupContext := func(ctx context.Context) context.Context {
		ctx = podlog.NewLoaderContext(ctx, podLogStreamer)
		ctx = application.NewLoaderContext(ctx, watchers.AppWatcher, watchers.IngressWatcher, prometheusClient, log)
		ctx = bigquery.NewLoaderContext(ctx, watchers.BqWatcher)
		ctx = bucket.NewLoaderContext(ctx, watchers.BucketWatcher)
		ctx = job.NewLoaderContext(ctx, watchers.JobWatcher, watchers.RunWatcher)
		ctx = kafkatopic.NewLoaderContext(ctx, watchers.KafkaTopicWatcher)
		ctx = workload.NewLoaderContext(ctx, watchers.PodWatcher)
		ctx = secret.NewLoaderContext(ctx, secretClientCreator, clusters, log)
		ctx = aiven.NewLoaderContext(ctx, aivenProjects)
		ctx = opensearch.NewLoaderContext(ctx, tenantName, watchers.OpenSearchWatcher, aivenClient, log)
		ctx = valkey.NewLoaderContext(ctx, tenantName, watchers.ValkeyWatcher, aivenClient)
		ctx = price.NewLoaderContext(ctx, priceRetriever, log)
		ctx = utilization.NewLoaderContext(ctx, prometheusClient, log)
		ctx = alerts.NewLoaderContext(ctx, prometheusClient, log)
		ctx = metrics.NewLoaderContext(ctx, prometheusClient, log)
		ctx = sqlinstance.NewLoaderContext(ctx, sqlAdminService, watchers.SqlDatabaseWatcher, watchers.SqlInstanceWatcher, auditLogProjectID, auditLogLocation)
		ctx = database.NewLoaderContext(ctx, pool)
		ctx = issue.NewContext(ctx, pool)
		ctx = team.NewLoaderContext(ctx, pool, watchers.NamespaceWatcher)
		ctx = loki.NewLoaderContext(ctx, lokiClient)
		ctx = user.NewLoaderContext(ctx, pool)
		ctx = usersync.NewLoaderContext(ctx, pool)
		ctx = cost.NewLoaderContext(ctx, pool, costOpts...)
		ctx = repository.NewLoaderContext(ctx, pool)
		ctx = authz.NewLoaderContext(ctx, pool)
		ctx = activitylog.NewLoaderContext(ctx, pool)
		ctx = vulnerability.NewLoaderContext(ctx, vulnMgr, log)
		ctx = servicemaintenance.NewLoaderContext(ctx, serviceMaintenanceManager, log)
		ctx = reconciler.NewLoaderContext(ctx, pool)
		ctx = deployment.NewLoaderContext(ctx, pool, hookdClient)
		ctx = serviceaccount.NewLoaderContext(ctx, pool)
		ctx = session.NewLoaderContext(ctx, pool)
		ctx = search.NewLoaderContext(ctx, pool, searcher)
		ctx = unleash.NewLoaderContext(ctx, tenantName, watchers.UnleashWatcher, bifrostAPIURL, allowedClusters, log)
		ctx = logging.NewPackageContext(ctx, tenantName, defaultLogDestinations)
		ctx = environment.NewLoaderContext(ctx, pool)
		ctx = feature.NewLoaderContext(
			ctx,
			watchers.UnleashWatcher.Enabled(),
			watchers.ValkeyWatcher.Enabled(),
			watchers.KafkaTopicWatcher.Enabled(),
			watchers.OpenSearchWatcher.Enabled(),
		)
		return ctx
	}

	return loader.Middleware(setupContext), nil
}
