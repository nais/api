//go:build integration_test

package integration

type Config struct {
	TenantName         string            `yaml:"tenant_name"`
	EnvironmentMapping map[string]string `yaml:"environment_mapping"`
}

func newConfig() any {
	return &Config{
		TenantName: "some-tenant",
	}
}
