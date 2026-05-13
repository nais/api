package job

import (
	"context"
	"testing"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload"
)

func TestSortFilter_NextRun(t *testing.T) {
	makeJob := func(name, schedule string) *Job {
		return &Job{
			Base: workload.Base{Name: name},
			Spec: &nais_io_v1.NaisjobSpec{Schedule: schedule},
		}
	}

	t.Run("ASC sorts by next run time", func(t *testing.T) {
		jobs := []*Job{
			makeJob("rare", "0 0 1 1 *"),
			makeJob("frequent", "* * * * *"),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)

		if jobs[0].Name != "frequent" || jobs[1].Name != "rare" {
			t.Fatalf("expected [frequent, rare], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("DESC sorts by next run time descending", func(t *testing.T) {
		jobs := []*Job{
			makeJob("frequent", "* * * * *"),
			makeJob("rare", "0 0 1 1 *"),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionDesc)

		if jobs[0].Name != "rare" || jobs[1].Name != "frequent" {
			t.Fatalf("expected [rare, frequent], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("ASC puts unscheduled jobs last", func(t *testing.T) {
		jobs := []*Job{
			makeJob("noSchedule", ""),
			makeJob("scheduled", "* * * * *"),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)

		if jobs[0].Name != "scheduled" || jobs[1].Name != "noSchedule" {
			t.Fatalf("expected [scheduled, noSchedule], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("DESC puts unscheduled jobs last with partition", func(t *testing.T) {
		jobs := []*Job{
			makeJob("noSchedule", ""),
			makeJob("scheduled", "* * * * *"),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionDesc)
		partitionUnscheduledLast(jobs)

		if jobs[0].Name != "scheduled" || jobs[1].Name != "noSchedule" {
			t.Fatalf("expected [scheduled, noSchedule], got [%s, %s]", jobs[0].Name, jobs[1].Name)
		}
	})

	t.Run("invalid cron jobs sorted last like unscheduled", func(t *testing.T) {
		jobs := []*Job{
			makeJob("badCron", "invalid"),
			makeJob("good", "* * * * *"),
			makeJob("noSchedule", ""),
		}

		SortFilter.Sort(context.Background(), jobs, "NEXT_RUN", model.OrderDirectionAsc)
		partitionUnscheduledLast(jobs)

		if jobs[0].Name != "good" {
			t.Fatalf("expected good first, got %s", jobs[0].Name)
		}
		if jobs[1].Name == "good" || jobs[2].Name == "good" {
			t.Fatal("expected badCron and noSchedule to both be after good")
		}
	})
}
