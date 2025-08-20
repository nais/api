package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/team"
)

func (r *teamEnvironmentResolver) Alerts(ctx context.Context, obj *team.TeamEnvironment) ([]*alerts.Alerts, error) {
	panic(fmt.Errorf("not implemented: Alerts - alerts"))
}
