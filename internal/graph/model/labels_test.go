package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
)

func TestMergeUserLabels(t *testing.T) {
	existing := map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
		"nais.io/managed-by":           "console",
		"my-custom-key":                "testing",
	}

	desired := []*model.ResourceLabel{
		{Key: "another-key", Value: "val"},
	}

	got := model.MergeUserLabels(existing, desired)

	// Since app.kubernetes.io/managed-by was "Helm" (not "console"), it should be removed!
	if _, ok := got["app.kubernetes.io/managed-by"]; ok {
		t.Errorf("expected app.kubernetes.io/managed-by: Helm to be removed, but it was kept")
	}

	// nais.io/managed-by is reserved/hidden, so it must be kept
	if val := got["nais.io/managed-by"]; val != "console" {
		t.Errorf("expected nais.io/managed-by: console to be kept, but got %q", val)
	}

	// my-custom-key is a user label and not in desired, so it should be removed
	if _, ok := got["my-custom-key"]; ok {
		t.Errorf("expected my-custom-key to be removed, but it was kept")
	}

	// another-key was in desired, so it must be added
	if val := got["another-key"]; val != "val" {
		t.Errorf("expected another-key: val, but got %q", val)
	}
}

func TestUserLabels(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
		"nais.io/managed-by":           "console",
		"my-custom-key":                "testing",
	}

	got := model.UserLabels(labels)

	if len(got) != 2 {
		t.Errorf("expected 2 user labels, got %d", len(got))
	}

	// app.kubernetes.io/managed-by: Helm should be exposed to user
	foundHelm := false
	for _, l := range got {
		if l.Key == "app.kubernetes.io/managed-by" && l.Value == "Helm" {
			foundHelm = true
		}
	}
	if !foundHelm {
		t.Errorf("expected app.kubernetes.io/managed-by: Helm to be returned as user label")
	}
}
