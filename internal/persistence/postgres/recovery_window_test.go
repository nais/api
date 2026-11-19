package postgres

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFirstRecoverabilityPoint(t *testing.T) {
	want := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	objectStore := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"serverRecoveryWindow": map[string]any{
				"pg-orders-main": map[string]any{
					"firstRecoverabilityPoint": want.Format(time.RFC3339),
				},
			},
		},
	}}
	got, err := firstRecoverabilityPoint(objectStore, "pg-orders-main")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Equal(want) {
		t.Fatalf("firstRecoverabilityPoint = %v, want %v", got, want)
	}

	got, err = firstRecoverabilityPoint(objectStore, "missing-cluster")
	if err != nil || got != nil {
		t.Errorf("missing recovery window = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestBarmanObjectStoreName(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"plugins": []any{
			map[string]any{"name": "barman-cloud.cloudnative-pg.io", "parameters": map[string]any{"barmanObjectName": "orders-store"}},
		}},
	}}
	got, err := barmanObjectStoreName(cluster)
	if err != nil || got != "orders-store" {
		t.Fatalf("barmanObjectStoreName = (%q, %v), want (%q, nil)", got, err, "orders-store")
	}
}
