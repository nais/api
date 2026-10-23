package secret

import (
	"encoding/json"
	"testing"

	"github.com/nais/api/internal/activitylog"
)

func TestSecretActivityDataJSON(t *testing.T) {
	for _, tt := range []struct {
		name   string
		data   any
		legacy string
		want   string
	}{
		{
			name:   "updated fields",
			data:   &SecretUpdatedActivityLogEntryData{},
			legacy: `{"UpdatedFields":[{"Field":"data.API_KEY","OldValue":null,"NewValue":null}]}`,
			want:   `{"updatedFields":[{"field":"data.API_KEY","oldValue":null,"newValue":null}]}`,
		},
		{
			name:   "value added",
			data:   &SecretValueAddedActivityLogEntryData{},
			legacy: `{"ValueName":"API_KEY"}`,
			want:   `{"valueName":"API_KEY"}`,
		},
		{
			name:   "value updated",
			data:   &SecretValueUpdatedActivityLogEntryData{},
			legacy: `{"ValueName":"API_KEY"}`,
			want:   `{"valueName":"API_KEY"}`,
		},
		{
			name:   "value removed",
			data:   &SecretValueRemovedActivityLogEntryData{},
			legacy: `{"ValueName":"API_KEY"}`,
			want:   `{"valueName":"API_KEY"}`,
		},
		{
			name:   "values viewed",
			data:   &SecretValuesViewedActivityLogEntryData{},
			legacy: `{"Reason":"Debugging","ElevationID":"elevation-123"}`,
			want:   `{"reason":"Debugging","elevationID":"elevation-123"}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.legacy), tt.data); err != nil {
				t.Fatal(err)
			}
			data, err := activitylog.MarshalData(activitylog.CreateInput{Data: tt.data})
			if err != nil {
				t.Fatal(err)
			}
			if got := string(data); got != tt.want {
				t.Errorf("payload = %s, want %s", got, tt.want)
			}
		})
	}
}
