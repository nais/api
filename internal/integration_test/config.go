//go:build integration_test

package integration_test

type Config struct {
	SkipSeed        bool `yaml:"skip_seed"`
	Unauthenticated bool `yaml:"unauthenticated"`
	Admin           bool `yaml:"admin"`
}
