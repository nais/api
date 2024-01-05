package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

const (
	labelDataloader = "dataloader"
)

var (
	dataloaderLoads = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "dataloader_loads",
		Help:      "Dataloader loads",
	}, []string{labelDataloader})

	dataloaderHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "dataloader_hits",
		Help:      "Dataloader hits",
	}, []string{labelDataloader})

	dataloaderCacheClears = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "dataloader_cache_clears",
		Help:      "Dataloader cache clears",
	}, []string{labelDataloader})
)

func inc(vec *prometheus.CounterVec, labelValues ...string) {
	m, err := vec.GetMetricWithLabelValues(labelValues...)
	if err != nil {
		log.Warnf("get metric: %v", err)
	} else {
		m.Inc()
	}
}

func IncDataloaderLoads(dataloader string) {
	inc(dataloaderLoads, dataloader)
}

func IncDataloaderCalls(dataloader string) {
	inc(dataloaderHits, dataloader)
}

func IncDataloaderCacheClears(dataloader string) {
	inc(dataloaderCacheClears, dataloader)
}
