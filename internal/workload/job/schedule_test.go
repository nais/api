package job

import (
	"testing"
	"time"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func TestJob_Schedule(t *testing.T) {
	t.Run("empty schedule returns nil", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: ""}}
		if j.Schedule() != nil {
			t.Fatal("expected nil schedule")
		}
	})

	t.Run("valid cron expression returns future nextRun", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: "0 * * * *"}}
		s := j.Schedule()
		if s == nil {
			t.Fatal("expected non-nil schedule")
		}
		if s.Expression != "0 * * * *" {
			t.Fatalf("unexpected expression: %s", s.Expression)
		}
		if s.TimeZone != "UTC" {
			t.Fatalf("unexpected timezone: %s", s.TimeZone)
		}
		if s.NextRun == nil {
			t.Fatal("expected non-nil nextRun")
		}
		if !s.NextRun.After(time.Now()) {
			t.Fatal("expected nextRun to be in the future")
		}
	})

	t.Run("custom timezone is respected", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{
			Schedule: "0 12 * * *",
			TimeZone: new("Europe/Oslo"),
		}}
		s := j.Schedule()
		if s == nil {
			t.Fatal("expected non-nil schedule")
		}
		if s.TimeZone != "Europe/Oslo" {
			t.Fatalf("unexpected timezone: %s", s.TimeZone)
		}
		if s.NextRun == nil {
			t.Fatal("expected non-nil nextRun")
		}
	})

	t.Run("invalid timezone falls back to UTC", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{
			Schedule: "0 * * * *",
			TimeZone: new("Invalid/Zone"),
		}}
		s := j.Schedule()
		if s == nil {
			t.Fatal("expected non-nil schedule")
		}
		if s.TimeZone != "UTC" {
			t.Fatalf("expected timezone to be UTC, got %s", s.TimeZone)
		}
		if s.NextRun == nil {
			t.Fatal("expected non-nil nextRun even with invalid timezone")
		}
	})

	t.Run("invalid cron expression returns nil nextRun", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: "not a cron"}}
		s := j.Schedule()
		if s == nil {
			t.Fatal("expected non-nil schedule")
		}
		if s.Expression != "not a cron" {
			t.Fatalf("unexpected expression: %s", s.Expression)
		}
		if s.NextRun != nil {
			t.Fatalf("expected nil nextRun for invalid cron, got %v", s.NextRun)
		}
	})
}
