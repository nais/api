package vulnerabilities

import (
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	Client
}

type Config struct {
	DependencyTrack DependencyTrackConfig
}

func NewManager(cfg *Config) *Manager {
	dependencytrackClient := NewDependencyTrackClient(
		cfg.DependencyTrack,
		log.WithField("client", "dependencytrack"),
	)
	return &Manager{
		Client: dependencytrackClient,
	}
}
