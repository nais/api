package unleash

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
)

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
