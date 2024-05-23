package redis

import (
	"context"
	"github.com/nais/api/internal/slug"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database"
	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	log      log.FieldLogger
	costRepo database.CostRepo
}

func (m *Metrics) CostForRedisInstance(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := m.costRepo.CostForInstance(ctx, "Redis", from, to, teamSlug, ownerName, env); err != nil {
		m.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}
