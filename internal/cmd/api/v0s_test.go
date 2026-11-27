package api

import (
	"testing"

	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/v0s/fake"
)

func TestV0sManagerLocalFallback(t *testing.T) {
	for _, tt := range []struct {
		name        string
		fakeK8s     bool
		endpoint    string
		wantFake    bool
		wantEnabled bool
	}{
		{name: "local without v0s", fakeK8s: true, wantFake: true, wantEnabled: true},
		{name: "explicit endpoint overrides local fake", fakeK8s: true, endpoint: "localhost:12345", wantEnabled: true},
		{name: "production with endpoint", endpoint: "localhost:12345", wantEnabled: true},
		{name: "production still requires endpoint"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Fakes: Fakes{WithFakeKubernetes: tt.fakeK8s},
				V0s:   v0sConfig{Endpoint: tt.endpoint, SuppressionsEnabled: true},
			}
			manager, err := newV0sManager(cfg, &watchers.Watchers{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, fakeClient := manager.Vulnerabilities.(*fake.Client); fakeClient != tt.wantFake {
				t.Errorf("fake client = %v, want %v", fakeClient, tt.wantFake)
			}
			if enabled := manager.Vulnerabilities != nil && manager.Suppressions != nil; enabled != tt.wantEnabled {
				t.Errorf("v0s enabled = %v, want %v", enabled, tt.wantEnabled)
			}
			if !manager.SuppressionsEnabled {
				t.Error("suppression configuration was not preserved")
			}
		})
	}
}
