//go:build integration_test

package integration

type Config struct {
	TenantName             string `yaml:"tenant_name"`
	V0s                    bool   `yaml:"v0s"`
	V0sSuppressionsEnabled bool   `yaml:"v0s_suppressions_enabled"`
}

func newConfig() any {
	return &Config{
		TenantName: "some-tenant",
	}
}
