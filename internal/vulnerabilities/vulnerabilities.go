package vulnerabilities

import (
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	Client
}

type Config struct {
	EnableFakes     bool
	DependencyTrack DependencyTrackConfig
}

func NewManager(cfg *Config) *Manager {
	dependencytrackClient := NewDependencyTrackClient(
		cfg.DependencyTrack,
		log.WithField("client", "dependencytrack"),
	)
	if cfg.EnableFakes {
		dependencytrackClient = NewFake(dependencytrackClient)
	}
	return &Manager{
		Client: dependencytrackClient,
	}
}
