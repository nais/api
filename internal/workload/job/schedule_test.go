package job

import (
	"math"
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
		if s.NextRun.IsZero() {
			t.Fatal("expected non-zero nextRun")
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
		if s.NextRun.IsZero() {
			t.Fatal("expected non-zero nextRun")
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
		if s.NextRun.IsZero() {
			t.Fatal("expected non-zero nextRun even with invalid timezone")
		}
	})

	t.Run("invalid cron expression returns nil", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: "not a cron"}}
		if j.Schedule() != nil {
			t.Fatal("expected nil schedule for invalid cron expression")
		}
	})
}

func TestNextRunUnix(t *testing.T) {
	t.Run("nil schedule returns MaxInt64", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: ""}}
		if got := nextRunUnix(j); got != math.MaxInt64 {
			t.Fatalf("expected MaxInt64, got %d", got)
		}
	})

	t.Run("invalid cron returns MaxInt64", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: "bad"}}
		if got := nextRunUnix(j); got != math.MaxInt64 {
			t.Fatalf("expected MaxInt64 for zero nextRun, got %d", got)
		}
	})

	t.Run("valid schedule returns unix timestamp", func(t *testing.T) {
		j := &Job{Spec: &nais_io_v1.NaisjobSpec{Schedule: "0 * * * *"}}
		got := nextRunUnix(j)
		if got == math.MaxInt64 {
			t.Fatal("expected a real timestamp, got MaxInt64")
		}
		if got <= time.Now().Unix() {
			t.Fatalf("expected future timestamp, got %d", got)
		}
	})
}
