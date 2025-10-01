package log

import (
	"time"

	"github.com/nais/api/internal/slug"
)

type LogLine struct {
	Team        slug.Slug `json:"team"`
	Environment string    `json:"environment"`
	Time        time.Time `json:"time"`
	Message     string    `json:"message"`
	Application *string   `json:"application,omitempty"`
}

type LogSubscriptionFilter struct {
	Team        slug.Slug `json:"team"`
	Environment string    `json:"environment"`
	Application *string   `json:"application"`
	Job         *string   `json:"job"`
	Instances   []string  `json:"instances"`
}
