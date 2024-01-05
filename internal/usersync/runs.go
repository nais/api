package usersync

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type (
	RunsHandler struct {
		maxEntries int
		runs       []*Run
		lock       sync.RWMutex
	}
	RunStatus int64
	Run       struct {
		startedAt     time.Time
		finishedAt    *time.Time
		correlationID uuid.UUID
		status        RunStatus
		err           error
		lock          sync.RWMutex
	}
)

const (
	RunInProgress RunStatus = iota
	RunSuccess
	RunFailure
)

func NewRunsHandler(maxEntries int) *RunsHandler {
	return &RunsHandler{
		maxEntries: maxEntries,
		runs:       make([]*Run, 0),
	}
}

func (r *RunsHandler) StartNewRun(correlationID uuid.UUID) *Run {
	r.lock.Lock()
	defer r.lock.Unlock()

	run := &Run{
		correlationID: correlationID,
		startedAt:     time.Now(),
		status:        RunInProgress,
	}

	r.runs = append([]*Run{run}, r.runs...)
	if len(r.runs) > r.maxEntries {
		r.runs = r.runs[:r.maxEntries]
	}

	return run
}

func (r *RunsHandler) GetRuns() []*Run {
	return r.runs
}

func (r *Run) Finish() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.finishedAt != nil {
		return
	}

	now := time.Now()
	r.finishedAt = &now
	r.status = RunSuccess
}

func (r *Run) FinishWithError(err error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	now := time.Now()
	r.finishedAt = &now
	r.status = RunFailure
	r.err = err
}

func (r *Run) Status() RunStatus {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.status
}

func (r *Run) StartedAt() time.Time {
	return r.startedAt
}

func (r *Run) FinishedAt() *time.Time {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.finishedAt
}

func (r *Run) CorrelationID() uuid.UUID {
	return r.correlationID
}

func (r *Run) Error() error {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.err
}
