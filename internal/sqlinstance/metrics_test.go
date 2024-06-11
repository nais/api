package sqlinstance

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/sqlinstance/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	f, err := fake.NewFakeGoogleAPI(fake.WithInstanceLister(sqlInstances))
	if err != nil {
		t.Errorf("Failed to create fake google api: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m, err := NewMetrics(ctx, nil, logrus.WithField("test", "metrics"), f.ClientGRPCOptions...)
	assert.NoError(t, err)

	metrics, err := m.metricsForSqlInstance(ctx, &model.SQLInstance{
		Name:      "test",
		ProjectID: "project",
	})
	assert.NoError(t, err)

	assert.Equal(t, float64(1), metrics.CPU.Cores)
	assert.Equal(t, float64(50), metrics.CPU.Utilization)
	assert.Equal(t, float64(80), metrics.Memory.Utilization)
	assert.Equal(t, 4000000000, metrics.Memory.QuotaBytes)
	assert.Equal(t, float64(30), metrics.Disk.Utilization)
	assert.Equal(t, 1000000000, metrics.Disk.QuotaBytes)
}

func sqlInstances() ([]*model.SQLInstance, error) {
	return []*model.SQLInstance{
		{
			Name:      "test",
			ProjectID: "project",
		},
	}, nil
}
