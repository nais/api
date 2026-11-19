package job

import (
	"testing"

	"github.com/nais/api/internal/activitylog"
)

func TestJobRunActivityDataJSON(t *testing.T) {
	for _, tt := range []struct {
		name string
		data any
	}{
		{"triggered", &JobTriggeredActivityLogEntryData{RunName: "job-run-123"}},
		{"deleted", &JobRunDeletedActivityLogEntryData{RunName: "job-run-123"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := activitylog.MarshalData(activitylog.CreateInput{Data: tt.data})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(data), `{"runName":"job-run-123"}`; got != want {
				t.Errorf("payload = %s, want %s", got, want)
			}
		})
	}
}

func TestJobRunDeletedActivityDataAcceptsLegacyFieldName(t *testing.T) {
	data, err := activitylog.UnmarshalData[JobRunDeletedActivityLogEntryData](activitylog.GenericActivityLogEntry{
		Data: []byte(`{"RunName":"legacy-run"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if data.RunName != "legacy-run" {
		t.Errorf("RunName = %q, want legacy-run", data.RunName)
	}
}
