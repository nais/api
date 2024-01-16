package graph

import (
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/usersync"
)

func toGraphUserSyncRuns(runs []*usersync.Run) []*model.UserSyncRun {
	ret := make([]*model.UserSyncRun, len(runs))
	for i, run := range runs {
		ret[i] = &model.UserSyncRun{
			CorrelationID: run.CorrelationID(),
			StartedAt:     run.StartedAt(),
			FinishedAt:    run.FinishedAt(),
			GQLVars: model.UserSyncRunGQLVars{
				Status: run.Status(),
				Error:  run.Error(),
			},
		}
	}
	return ret
}
