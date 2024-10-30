package unleash

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ctxKey int

const (
	prometheusURL        = "https://nais-prometheus.%s.cloud.nais.io"
	loadersKey    ctxKey = iota
)

// NewLoaderContext creates a new context with a loaders value.
// If *fake* is provided as bifrostAPIURL, a fake client will be used.
func NewLoaderContext(ctx context.Context, tenantName string, appWatcher *watcher.Watcher[*UnleashInstance], bifrostAPIURL string, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(tenantName, appWatcher, bifrostAPIURL, log))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*UnleashInstance] {
	w := watcher.Watch(mgr, &UnleashInstance{}, watcher.WithGVR(unleash_nais_io_v1.GroupVersion.WithResource("unleashes")), watcher.WithConverter(unleashInstanceFromUnstructured))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	unleashWatcher *watcher.Watcher[*UnleashInstance]
	prometheus     Prometheus
	bifrostClient  BifrostClient
}

func newLoaders(tenantName string, appWatcher *watcher.Watcher[*UnleashInstance], bifrostAPIURL string, log logrus.FieldLogger) *loaders {
	var client BifrostClient
	var prometheus Prometheus
	if bifrostAPIURL == "*fake*" {
		client = NewFakeBifrostClient(appWatcher)
		prometheus = NewFakePrometheusClient()
	} else {
		client = NewBifrostClient(bifrostAPIURL, log)
		promClient, err := promapi.NewClient(promapi.Config{
			Address: fmt.Sprintf(prometheusURL, tenantName),
		})
		if err != nil {
			panic(fmt.Errorf("failed to create prometheus client: %w", err))
		}

		prometheus = promv1.NewAPI(promClient)
	}

	return &loaders{
		unleashWatcher: appWatcher,
		bifrostClient:  client,
		prometheus:     prometheus,
	}
}

func (l *loaders) PromQuery(ctx context.Context, q string) (model.SampleValue, error) {
	val, _, err := l.prometheus.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	switch val.Type() {
	case model.ValVector:
		if len(val.(model.Vector)) == 0 {
			return 0, nil
		}
		return val.(model.Vector)[0].Value, nil
	default:
		return 0, fmt.Errorf("unexpected PromQuery result type: %s", val.Type())
	}
}

func unleashInstanceFromUnstructured(o *unstructured.Unstructured, _ string) (obj any, ok bool) {
	unleashInstance := &unleash_nais_io_v1.Unleash{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, unleashInstance); err != nil {
		return nil, false
	}
	return toUnleashInstance(unleashInstance), true
}
