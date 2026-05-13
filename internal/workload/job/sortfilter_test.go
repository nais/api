package job

import (
	"context"
	"testing"
	"time"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload"
)

func TestSortFilter_NextRun(t *testing.T) {
	now := time.Now()
	soon := now.Add(1 * time.Hour)
	later := now.Add(2 * time.Hour)

	jobWithNextRun := func(name string, nextRun *time.Time) *Job {
		j := &Job{
			Base: workload.Base{Name: name},
			Spec: &nais_io_v1.NaisjobSpec{Schedule: "0 * * * *"},
		}
		if nextRun != nil {
			j.schedule = &JobSchedule{
				Expression: "0 * * * *",
				TimeZone:   "UTC",
				NextRun:    nextRun,
			}
			j.scheduleOnce.Do(func() {})
		}
		return j
	}

	jobWithoutSchedule := func(name string) *Job {
		j := &Job{
			Base: workload.Base{Name: name},
			Spec: &nais_io_v1.NaisjobSpec{},
		}
		return j
	}

	jobWithInvalidCron := func(name string) *Job {
		j := &Job{
			Base: workload.Base{Name: name},
			Spec: &nais_io_v1.NaisjobSpec{Schedule: "invalid"},
		}
		j.schedule = &JobSchedule{
			Expression: "invalid",
			TimeZone:   "UTC",
			NextRun:    nil,
		}
		j.scheduleOnce.Do(func() {})
		return j
	}

	t.Run("ASC sorts by next run time", func(t *testing.T) {
		jobs := []*Job{
			jobWithNextRun("later", &later),
			jobWithNextRun("soon", &soon),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)

		if jobs[0].Name != "soon" || jobs[1].Name != "later" {
			t.Fatalf("expected [soon, later], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("DESC sorts by next run time descending", func(t *testing.T) {
		jobs := []*Job{
			jobWithNextRun("soon", &soon),
			jobWithNextRun("later", &later),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionDesc)

		if jobs[0].Name != "later" || jobs[1].Name != "soon" {
			t.Fatalf("expected [later, soon], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("ASC puts unscheduled jobs last", func(t *testing.T) {
		jobs := []*Job{
			jobWithoutSchedule("noSchedule"),
			jobWithNextRun("scheduled", &soon),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)

		if jobs[0].Name != "scheduled" || jobs[1].Name != "noSchedule" {
			t.Fatalf("expected [scheduled, noSchedule], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("DESC puts unscheduled jobs last with partition", func(t *testing.T) {
		jobs := []*Job{
			jobWithoutSchedule("noSchedule"),
			jobWithNextRun("scheduled", &soon),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionDesc)
		partitionUnscheduledLast(jobs)

		if jobs[0].Name != "scheduled" || jobs[1].Name != "noSchedule" {
			t.Fatalf("expected [scheduled, noSchedule], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("invalid cron jobs sorted last like unscheduled", func(t *testing.T) {
		jobs := []*Job{
			jobWithInvalidCron("badCron"),
			jobWithNextRun("good", &soon),
			jobWithoutSchedule("noSchedule"),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)
		partitionUnscheduledLast(jobs)

		if jobs[0].Name != "good" {
			t.Fatalf("expected good first, got %s", jobs[0].Name)
		}
		if jobs[1].Name != "badCron" && jobs[2].Name != "badCron" {
			t.Fatal("expected badCron to be sorted last along with noSchedule")
		}
	})
}
