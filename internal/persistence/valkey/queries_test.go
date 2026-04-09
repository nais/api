package valkey

import (
	"testing"
)

func TestValkeyEnvVarSuffix(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		want         string
	}{
		{name: "simple name", instanceName: "foo", want: "FOO"},
		{name: "hyphenated name", instanceName: "my-cache", want: "MY_CACHE"},
		{name: "multiple hyphens", instanceName: "my-valkey-instance", want: "MY_VALKEY_INSTANCE"},
		{name: "already uppercase chars", instanceName: "MyCache", want: "_Y_ACHE"},
		{name: "numbers", instanceName: "cache1", want: "CACHE1"},
		{name: "mixed", instanceName: "my-cache-2", want: "MY_CACHE_2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valkeyEnvVarSuffix(tt.instanceName)
			if got != tt.want {
				t.Errorf("valkeyEnvVarSuffix(%q) = %q, want %q", tt.instanceName, got, tt.want)
			}
		})
	}
}
