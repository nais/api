package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
)

func toGraphReconciler(m *database.Reconciler) *model.Reconciler {
	return &model.Reconciler{
		Name:        string(m.Name),
		DisplayName: m.DisplayName,
		Description: m.Description,
		Enabled:     m.Enabled,
		RunOrder:    int(m.RunOrder),
	}
}
