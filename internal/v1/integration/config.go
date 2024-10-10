package integration

type Config struct {
	SkipSeed        bool `yaml:"skip_seed"`
	Unauthenticated bool `yaml:"unauthenticated"`
	Admin           bool `yaml:"admin"`
}

func newConfig() any {
	return &Config{}
}
