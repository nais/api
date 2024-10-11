package podlog

import (
	"strings"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

type WorkloadLogLine struct {
	Time     time.Time `json:"time"`
	Message  string    `json:"message"`
	Instance string    `json:"instance"`
}

type WorkloadLogSubscriptionFilter struct {
	Team        slug.Slug `json:"team"`
	Environment string    `json:"environment"`
	Application *string   `json:"application"`
	Job         *string   `json:"job"`
	Instances   []string  `json:"instances"`
}

func (filter *WorkloadLogSubscriptionFilter) Sanitized() *WorkloadLogSubscriptionFilter {
	sanitized := &WorkloadLogSubscriptionFilter{
		Team:        filter.Team,
		Environment: strings.TrimSpace(filter.Environment),
		Instances: func(instances []string) []string {
			var sanitized []string
			for _, instance := range instances {
				if instance = strings.TrimSpace(instance); instance != "" {
					sanitized = append(sanitized, instance)
				}
			}
			return sanitized
		}(filter.Instances),
	}

	if filter.Application != nil {
		sanitized.Application = ptr.To(strings.TrimSpace(*filter.Application))
	}

	if filter.Job != nil {
		sanitized.Job = ptr.To(strings.TrimSpace(*filter.Job))
	}

	return sanitized
}

func (filter *WorkloadLogSubscriptionFilter) Validate() error {
	if (filter.Application == nil && filter.Job == nil) || (filter.Application != nil && filter.Job != nil) {
		return apierror.Errorf("You must filter on either application or a job.")
	}

	if filter.Application != nil && *filter.Application == "" {
		return apierror.Errorf("Application cannot be empty.")
	}

	if filter.Job != nil && *filter.Job == "" {
		return apierror.Errorf("Job cannot be empty.")
	}

	return nil
}
