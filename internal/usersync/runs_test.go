package usersync_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/usersync"
)

func TestRuns(t *testing.T) {
	runs := usersync.NewRunsHandler(5)
	if len(runs.GetRuns()) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs.GetRuns()))
	}

	ids := make([]uuid.UUID, 0)
	for range 10 {
		id := uuid.New()
		ids = append(ids, id)
		_ = runs.StartNewRun(id)
	}
	allRuns := runs.GetRuns()
	if len(allRuns) != 5 {
		t.Fatalf("expected 5 runs, got %d", len(allRuns))
	}

	if allRuns[0].CorrelationID() != ids[9] {
		t.Errorf("expected run 1 to have correlation id %v, got %v", ids[9], allRuns[0].CorrelationID())
	}

	if allRuns[1].CorrelationID() != ids[8] {
		t.Errorf("expected run 2 to have correlation id %v, got %v", ids[8], allRuns[1].CorrelationID())
	}

	if allRuns[2].CorrelationID() != ids[7] {
		t.Errorf("expected run 3 to have correlation id %v, got %v", ids[7], allRuns[2].CorrelationID())
	}

	if allRuns[3].CorrelationID() != ids[6] {
		t.Errorf("expected run 4 to have correlation id %v, got %v", ids[6], allRuns[3].CorrelationID())
	}

	if allRuns[4].CorrelationID() != ids[5] {
		t.Errorf("expected run 5 to have correlation id %v, got %v", ids[5], allRuns[4].CorrelationID())
	}
}

func TestRun(t *testing.T) {
	correlationID := uuid.New()
	runs := usersync.NewRunsHandler(5)

	t.Run("default values", func(t *testing.T) {
		run := runs.StartNewRun(correlationID)

		if run.CorrelationID() != correlationID {
			t.Errorf("expected correlation id %v, got %v", correlationID, run.CorrelationID())
		}

		if run.Status() != usersync.RunInProgress {
			t.Errorf("expected status %v, got %v", usersync.RunInProgress, run.Status())
		}

		if run.StartedAt().IsZero() {
			t.Errorf("expected started at to be set, got zero value")
		}

		if run.Error() != nil {
			t.Errorf("expected error to be nil, got %v", run.Error())
		}

		if run.FinishedAt() != nil {
			t.Errorf("expected finished at to be nil, got %v", run.FinishedAt())
		}
	})

	t.Run("success", func(t *testing.T) {
		run := runs.StartNewRun(correlationID)
		run.Finish()

		if run.CorrelationID() != correlationID {
			t.Errorf("expected correlation id %v, got %v", correlationID, run.CorrelationID())
		}

		if run.Status() != usersync.RunSuccess {
			t.Errorf("expected status %v, got %v", usersync.RunSuccess, run.Status())
		}

		if run.Error() != nil {
			t.Errorf("expected error to be nil, got %v", run.Error())
		}

		if run.FinishedAt().IsZero() {
			t.Errorf("expected finished at to be set, got zero value")
		}
	})

	t.Run("failure", func(t *testing.T) {
		err := fmt.Errorf("some error")
		run := runs.StartNewRun(correlationID)
		run.FinishWithError(err)

		if run.CorrelationID() != correlationID {
			t.Errorf("expected correlation id %v, got %v", correlationID, run.CorrelationID())
		}

		if run.Status() != usersync.RunFailure {
			t.Errorf("expected status %v, got %v", usersync.RunFailure, run.Status())
		}

		if !errors.Is(run.Error(), err) {
			t.Errorf("expected error to be %v, got %v", err, run.Error())
		}

		if run.FinishedAt().IsZero() {
			t.Errorf("expected finished at to be set, got zero value")
		}
	})
}
