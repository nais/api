package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

const DirtyLabel = "dirty"

var schemaVersion = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: namespace,
	Subsystem: subsystem,
	Name:      "schema_version",
	Help:      "Current database schema version",
}, []string{DirtyLabel})

func SetSchemaVersion(version uint, dirty bool) {
	m, err := schemaVersion.GetMetricWithLabelValues(strconv.FormatBool(dirty))
	if err != nil {
		log.Warnf("get metric: %v", err)
	} else {
		m.Set(float64(version))
	}
}
