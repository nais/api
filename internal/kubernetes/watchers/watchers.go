package watchers

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

type AppWatcher = watcher.Watcher[*nais_io_v1alpha1.Application]
type JobWatcher = watcher.Watcher[*nais_io_v1.Naisjob]
type RunWatcher = watcher.Watcher[*batchv1.Job]
type BqWatcher = watcher.Watcher[*bigquery.BigQueryDataset]
type ValkeyWatcher = watcher.Watcher[*valkey.Valkey]
type OpenSearchWatcher = watcher.Watcher[*opensearch.OpenSearch]
type BucketWatcher = watcher.Watcher[*bucket.Bucket]
type SqlDatabaseWatcher = watcher.Watcher[*sqlinstance.SQLDatabase]
type SqlInstanceWatcher = watcher.Watcher[*sqlinstance.SQLInstance]
type KafkaTopicWatcher = watcher.Watcher[*kafkatopic.KafkaTopic]
type PodWatcher = watcher.Watcher[*v1.Pod]
type IngressWatcher = watcher.Watcher[*netv1.Ingress]
type NamespaceWatcher = watcher.Watcher[*v1.Namespace]
type UnleashWatcher = watcher.Watcher[*unleash.UnleashInstance]

type Watchers struct {
	AppWatcher         *AppWatcher
	JobWatcher         *JobWatcher
	RunWatcher         *RunWatcher
	BqWatcher          *BqWatcher
	ValkeyWatcher      *ValkeyWatcher
	OpenSearchWatcher  *OpenSearchWatcher
	BucketWatcher      *BucketWatcher
	SqlDatabaseWatcher *SqlDatabaseWatcher
	SqlInstanceWatcher *SqlInstanceWatcher
	KafkaTopicWatcher  *KafkaTopicWatcher
	PodWatcher         *PodWatcher
	IngressWatcher     *IngressWatcher
	NamespaceWatcher   *NamespaceWatcher
	UnleashWatcher     *UnleashWatcher
}

func SetupWatchers(
	ctx context.Context,
	watcherMgr *watcher.Manager,
	mgmtWatcherMgr *watcher.Manager,
) *Watchers {
	return &Watchers{
		AppWatcher:         application.NewWatcher(ctx, watcherMgr),
		JobWatcher:         job.NewWatcher(ctx, watcherMgr),
		RunWatcher:         job.NewRunWatcher(ctx, watcherMgr),
		BqWatcher:          bigquery.NewWatcher(ctx, watcherMgr),
		ValkeyWatcher:      valkey.NewWatcher(ctx, watcherMgr),
		OpenSearchWatcher:  opensearch.NewWatcher(ctx, watcherMgr),
		BucketWatcher:      bucket.NewWatcher(ctx, watcherMgr),
		SqlDatabaseWatcher: sqlinstance.NewDatabaseWatcher(ctx, watcherMgr),
		SqlInstanceWatcher: sqlinstance.NewInstanceWatcher(ctx, watcherMgr),
		KafkaTopicWatcher:  kafkatopic.NewWatcher(ctx, watcherMgr),
		PodWatcher:         workload.NewWatcher(ctx, watcherMgr),
		IngressWatcher:     application.NewIngressWatcher(ctx, watcherMgr),
		NamespaceWatcher:   team.NewNamespaceWatcher(ctx, watcherMgr),
		UnleashWatcher:     unleash.NewWatcher(ctx, mgmtWatcherMgr),
	}
}
