package integration

type Config struct {
	SkipSeed        bool   `yaml:"skip_seed"`
	Unauthenticated bool   `yaml:"unauthenticated"`
	Admin           bool   `yaml:"admin"`
	TenantName      string `yaml:"tenant_name"`
}

func newConfig() any {
	return &Config{
		TenantName: "some-tenant",
	}
}
