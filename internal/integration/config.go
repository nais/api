//go:build integration_test

package integration

type Config struct {
	TenantName string `yaml:"tenant_name"`
	V0s        bool   `yaml:"v0s"`
}

func newConfig() any {
	return &Config{
		TenantName: "some-tenant",
	}
}
