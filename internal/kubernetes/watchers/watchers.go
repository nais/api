package watchers

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/secret"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

type (
	AppWatcher             = watcher.Watcher[*nais_io_v1alpha1.Application]
	JobWatcher             = watcher.Watcher[*nais_io_v1.Naisjob]
	RunWatcher             = watcher.Watcher[*batchv1.Job]
	BqWatcher              = watcher.Watcher[*bigquery.BigQueryDataset]
	ValkeyWatcher          = watcher.Watcher[*valkey.Valkey]
	OpenSearchWatcher      = watcher.Watcher[*opensearch.OpenSearch]
	BucketWatcher          = watcher.Watcher[*bucket.Bucket]
	SqlDatabaseWatcher     = watcher.Watcher[*sqlinstance.SQLDatabase]
	SqlInstanceWatcher     = watcher.Watcher[*sqlinstance.SQLInstance]
	ZalandoPostgresWatcher = watcher.Watcher[*postgres.Postgres]
	KafkaTopicWatcher      = watcher.Watcher[*kafkatopic.KafkaTopic]
	PodWatcher             = watcher.Watcher[*v1.Pod]
	IngressWatcher         = watcher.Watcher[*netv1.Ingress]
	NamespaceWatcher       = watcher.Watcher[*v1.Namespace]
	UnleashWatcher         = watcher.Watcher[*unleash.UnleashInstance]
	SecretWatcher          = watcher.Watcher[*secret.Secret]
)

type Watchers struct {
	AppWatcher             *AppWatcher
	JobWatcher             *JobWatcher
	RunWatcher             *RunWatcher
	BqWatcher              *BqWatcher
	ValkeyWatcher          *ValkeyWatcher
	OpenSearchWatcher      *OpenSearchWatcher
	BucketWatcher          *BucketWatcher
	SqlDatabaseWatcher     *SqlDatabaseWatcher
	SqlInstanceWatcher     *SqlInstanceWatcher
	ZalandoPostgresWatcher *ZalandoPostgresWatcher
	KafkaTopicWatcher      *KafkaTopicWatcher
	PodWatcher             *PodWatcher
	IngressWatcher         *IngressWatcher
	NamespaceWatcher       *NamespaceWatcher
	UnleashWatcher         *UnleashWatcher
	SecretWatcher          *SecretWatcher
}

func SetupWatchers(
	ctx context.Context,
	watcherMgr *watcher.Manager,
	mgmtWatcherMgr *watcher.Manager,
) *Watchers {
	return &Watchers{
		AppWatcher:             application.NewWatcher(ctx, watcherMgr),
		JobWatcher:             job.NewWatcher(ctx, watcherMgr),
		RunWatcher:             job.NewRunWatcher(ctx, watcherMgr),
		BqWatcher:              bigquery.NewWatcher(ctx, watcherMgr),
		ValkeyWatcher:          valkey.NewWatcher(ctx, watcherMgr),
		OpenSearchWatcher:      opensearch.NewWatcher(ctx, watcherMgr),
		BucketWatcher:          bucket.NewWatcher(ctx, watcherMgr),
		SqlDatabaseWatcher:     sqlinstance.NewDatabaseWatcher(ctx, watcherMgr),
		SqlInstanceWatcher:     sqlinstance.NewInstanceWatcher(ctx, watcherMgr),
		ZalandoPostgresWatcher: postgres.NewZalandoPostgresWatcher(ctx, watcherMgr),
		KafkaTopicWatcher:      kafkatopic.NewWatcher(ctx, watcherMgr),
		PodWatcher:             workload.NewWatcher(ctx, watcherMgr),
		IngressWatcher:         application.NewIngressWatcher(ctx, watcherMgr),
		NamespaceWatcher:       team.NewNamespaceWatcher(ctx, watcherMgr),
		UnleashWatcher:         unleash.NewWatcher(ctx, mgmtWatcherMgr),
		SecretWatcher:          secret.NewWatcher(ctx, watcherMgr),
	}
}
