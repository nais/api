package podlog

import (
	"context"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/validate"
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

func (f *WorkloadLogSubscriptionFilter) Validate(ctx context.Context) error {
	f.sanitize()
	verr := validate.New()

	if exists, err := team.Exists(ctx, f.Team); err != nil {
		return err
	} else if !exists {
		verr.Add("team", "Team does not exist.")
	}

	a := ptr.Deref(f.Application, "")
	j := ptr.Deref(f.Job, "")
	if (a == "" && j == "") || (a != "" && j != "") {
		verr.AddMessage("You must filter on either an application or a job.")
	}

	return verr.NilIfEmpty()
}

func (f *WorkloadLogSubscriptionFilter) sanitize() {
	f.Environment = strings.TrimSpace(f.Environment)
	f.Instances = func(instances []string) []string {
		var sanitized []string
		for _, instance := range instances {
			if instance = strings.TrimSpace(instance); instance != "" {
				sanitized = append(sanitized, instance)
			}
		}
		return sanitized
	}(f.Instances)

	if f.Application != nil {
		f.Application = ptr.To(strings.TrimSpace(*f.Application))
	}

	if f.Job != nil {
		f.Job = ptr.To(strings.TrimSpace(*f.Job))
	}
}
