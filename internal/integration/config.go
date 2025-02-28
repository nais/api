package integration

type Config struct {
	TenantName string `yaml:"tenant_name"`
}

func newConfig() any {
	return &Config{
		TenantName: "some-tenant",
	}
}
