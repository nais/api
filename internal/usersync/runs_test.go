package usersync_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/usersync"
	"github.com/stretchr/testify/assert"
)

func TestRuns(t *testing.T) {
	runs := usersync.NewRunsHandler(5)
	assert.Len(t, runs.GetRuns(), 0)
	ids := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		ids[i] = uuid.New()
		_ = runs.StartNewRun(ids[i])
	}
	allRuns := runs.GetRuns()
	assert.Len(t, allRuns, 5)
	assert.Equal(t, ids[9], allRuns[0].CorrelationID())
	assert.Equal(t, ids[8], allRuns[1].CorrelationID())
	assert.Equal(t, ids[7], allRuns[2].CorrelationID())
	assert.Equal(t, ids[6], allRuns[3].CorrelationID())
	assert.Equal(t, ids[5], allRuns[4].CorrelationID())
}

func TestRun(t *testing.T) {
	correlationID := uuid.New()
	runs := usersync.NewRunsHandler(5)

	t.Run("default values", func(t *testing.T) {
		run := runs.StartNewRun(correlationID)

		assert.Equal(t, correlationID, run.CorrelationID())
		assert.Equal(t, usersync.RunInProgress, run.Status())
		assert.NotNil(t, run.StartedAt())
		assert.Nil(t, run.Error())
		assert.Nil(t, run.FinishedAt())
	})

	t.Run("success", func(t *testing.T) {
		run := runs.StartNewRun(correlationID)
		run.Finish()

		assert.Equal(t, correlationID, run.CorrelationID())
		assert.Equal(t, usersync.RunSuccess, run.Status())
		assert.Nil(t, run.Error())
		assert.NotNil(t, run.FinishedAt())
	})

	t.Run("failure", func(t *testing.T) {
		err := fmt.Errorf("some error")
		run := runs.StartNewRun(correlationID)
		run.FinishWithError(err)

		assert.Equal(t, correlationID, run.CorrelationID())
		assert.Equal(t, usersync.RunFailure, run.Status())
		assert.Equal(t, err, run.Error())
		assert.NotNil(t, run.FinishedAt())
	})
}
