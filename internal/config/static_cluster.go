package config

import (
	"fmt"
	"strings"
)

type StaticCluster struct {
	Name  string
	Host  string
	Token string
}

func (c *StaticCluster) EnvDecode(value string) error {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return fmt.Errorf(`invalid static cluster entry: %q. Must be on format "name|host|token"`, value)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Name must not be empty", value)
	}

	host := strings.TrimSpace(parts[1])
	if host == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Host must not be empty", value)
	}

	token := strings.TrimSpace(parts[2])
	if token == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Token must not be empty", value)
	}

	*c = StaticCluster{
		Name:  name,
		Host:  host,
		Token: token,
	}
	return nil
}
