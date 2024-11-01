//go:build v0_test
// +build v0_test

package k8s_test

import (
	"fmt"
	"testing"

	"github.com/nais/api/internal/k8s"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

type TestCase struct {
	Job  Job
	Want string
}

type Job struct {
	Failed       int32
	BackoffLimit int32
	Active       int32
	Parallelism  int32
	Completions  int32
	Completed    int32
}

func TestRunMessage(t *testing.T) {
	for _, tc := range []TestCase{
		{
			Job{Failed: 2, BackoffLimit: 1},
			"Run failed after 2 attempts",
		},
		{
			Job{Completed: 2, Completions: 2, Failed: 0},
			"2/2 instances completed (0 failed attempts)",
		},
		{
			Job{Completed: 1, Completions: 1, Failed: 1},
			"1/1 instances completed (1 failed attempt)",
		},
		{
			Job{Completed: 6, Completions: 9, Parallelism: 3, Active: 3, Failed: 1},
			"3 instances running. 6/9 completed (1 failed attempt)",
		},
	} {
		t.Run(fmt.Sprintf("%v", tc.Job), func(t *testing.T) {
			got := k8s.Message(tc.Job.Convert())
			if got != tc.Want {
				t.Errorf("got: %q, want: %q", got, tc.Want)
			}
		})
	}
}

func (j *Job) Convert() *batchv1.Job {
	ret := &batchv1.Job{
		Spec: batchv1.JobSpec{
			Parallelism: &j.Parallelism,
			Completions: &j.Completions,
		},
		Status: batchv1.JobStatus{
			Failed:    j.Failed,
			Active:    j.Active,
			Succeeded: j.Completed,
		},
	}

	if j.BackoffLimit > 0 {
		ret.Spec.BackoffLimit = &j.BackoffLimit
	} else {
		bol := int32(6)
		ret.Spec.BackoffLimit = &bol
	}

	if ret.Status.Failed > *ret.Spec.BackoffLimit {
		ret.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: v1.ConditionTrue}}
	}

	return ret
}
