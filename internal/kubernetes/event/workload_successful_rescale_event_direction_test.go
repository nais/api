package event

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWorkloadSuccessfulRescaleEventDirection(t *testing.T) {
	for _, test := range []struct {
		stored string
		want   WorkloadSuccessfulRescaleEventDirection
	}{
		{`"up"`, WorkloadSuccessfulRescaleEventDirectionUp},
		{`"down"`, WorkloadSuccessfulRescaleEventDirectionDown},
		{`"UP"`, WorkloadSuccessfulRescaleEventDirectionUp},
		{`"DOWN"`, WorkloadSuccessfulRescaleEventDirectionDown},
	} {
		t.Run(test.stored, func(t *testing.T) {
			var direction WorkloadSuccessfulRescaleEventDirection
			if err := json.Unmarshal([]byte(test.stored), &direction); err != nil {
				t.Fatal(err)
			}
			if direction != test.want {
				t.Errorf("got direction %q, want %q", direction, test.want)
			}
			if !direction.IsValid() {
				t.Errorf("direction %q is not valid", direction)
			}
			var output bytes.Buffer
			direction.MarshalGQL(&output)
			if want := `"` + string(test.want) + `"`; output.String() != want {
				t.Errorf("got GraphQL value %s, want %s", output.String(), want)
			}
			if err := direction.UnmarshalGQL(string(test.want)); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, data := range []string{`"unknown"`, `""`, `null`, `42`} {
		var direction WorkloadSuccessfulRescaleEventDirection
		if err := json.Unmarshal([]byte(data), &direction); err == nil {
			t.Errorf("expected an error for JSON value %s", data)
		}
	}
	var direction WorkloadSuccessfulRescaleEventDirection
	if err := direction.UnmarshalGQL("up"); err == nil {
		t.Error("expected an error for lowercase GraphQL value")
	}
	if err := direction.UnmarshalGQL(42); err == nil {
		t.Error("expected an error for non-string GraphQL value")
	}
}
