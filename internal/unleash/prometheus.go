package unleash

import (
	"context"
	"fmt"
	"time"

	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Prometheus interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prom.Option) (model.Value, prom.Warnings, error)
}

func (m *Manager) PromQuery(ctx context.Context, q string) (model.SampleValue, error) {
	val, _, err := m.prometheus.Query(ctx, q, time.Now())
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
