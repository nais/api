package utilization

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/promclient"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	naisv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

type utilizationQueryClient struct {
	ResourceUsageClient
	queryAll func(string) (prom.Vector, error)
}

func (c *utilizationQueryClient) QueryAll(_ context.Context, query string, _ ...promclient.QueryOption) (prom.Vector, error) {
	return c.queryAll(query)
}

func TestForTeamExcludesMissingWorkloads(t *testing.T) {
	environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{"dev-gcp": "dev", "prod-gcp": "prod"})
	t.Cleanup(func() { environmentmapper.SetMapping(nil) })
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"dev-gcp": {}, "prod-gcp": {}}, logrus.New(),
		watcher.WithClientCreator(func(cluster string) (dynamic.Interface, watcher.KindResolver, *rest.Config, error) {
			if cluster == "prod-gcp" {
				return fake.NewSimpleDynamicClient(scheme,
					&naisv1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "pgbouncer", Namespace: "nais"}},
				), nil, &rest.Config{}, nil
			}
			return fake.NewSimpleDynamicClient(scheme,
				&naisv1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "console", Namespace: "nais"}},
				&naisv1.Naisjob{ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "nais"}},
				&naisv1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "other-team-app", Namespace: "other-team"}},
			), nil, &rest.Config{}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)
	ctx := application.NewLoaderContext(t.Context(), application.NewWatcher(t.Context(), manager), nil, logrus.New())
	ctx = job.NewLoaderContext(ctx, job.NewWatcher(t.Context(), manager), nil)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !manager.WaitForReady(waitCtx) {
		t.Fatal("timed out waiting for workload watchers")
	}

	for _, test := range []struct {
		resourceType UtilizationResourceType
		requestQuery string
		usageQuery   string
	}{
		{UtilizationResourceTypeCPU, TeamCPURequest, TeamCPUUsage},
		{UtilizationResourceTypeMemory, TeamMemoryRequest, TeamMemoryUsage},
	} {
		t.Run(test.resourceType.String(), func(t *testing.T) {
			teamSlug := slug.Slug("nais")
			calls := 0
			client := &utilizationQueryClient{
				queryAll: func(query string) (prom.Vector, error) {
					calls++
					var value prom.SampleValue
					switch query {
					case fmt.Sprintf(test.requestQuery, teamSlug, ignoredContainers):
						value = 10
					case fmt.Sprintf(test.usageQuery, teamSlug, ignoredContainers):
						value = 5
					default:
						return nil, fmt.Errorf("unexpected query: %s", query)
					}
					return prom.Vector{
						{Metric: prom.Metric{"container": "pgbouncer", "k8s_cluster_name": "dev-gcp"}, Value: 100},
						{Metric: prom.Metric{"container": "deleted-app", "k8s_cluster_name": "dev-gcp"}, Value: 100},
						{Metric: prom.Metric{"container": "console", "k8s_cluster_name": "prod-gcp"}, Value: 100},
						{Metric: prom.Metric{"container": "other-team-app", "k8s_cluster_name": "dev-gcp"}, Value: 100},
						{Metric: prom.Metric{"container": "console", "k8s_cluster_name": "dev-gcp"}, Value: value},
						{Metric: prom.Metric{"container": "cleanup", "k8s_cluster_name": "dev-gcp"}, Value: value},
						{Metric: prom.Metric{"container": "pgbouncer", "k8s_cluster_name": "prod-gcp"}, Value: value},
					}, nil
				},
			}
			ctx := NewLoaderContext(ctx, client, logrus.New())
			got, err := ForTeam(ctx, teamSlug, test.resourceType)
			if err != nil {
				t.Fatal(err)
			}
			want := []*WorkloadUtilizationData{{
				TeamSlug:        teamSlug,
				WorkloadName:    "console",
				EnvironmentName: "dev-gcp",
				Requested:       10,
				Used:            5,
			}, {
				TeamSlug:        teamSlug,
				WorkloadName:    "cleanup",
				EnvironmentName: "dev-gcp",
				Requested:       10,
				Used:            5,
			}, {
				TeamSlug:        teamSlug,
				WorkloadName:    "pgbouncer",
				EnvironmentName: "prod-gcp",
				Requested:       10,
				Used:            5,
			}}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("ForTeam() mismatch (-want +got):\n%s", diff)
			}
			if calls != 2 {
				t.Errorf("QueryAll() called %d times, want 2", calls)
			}
		})
	}
}
