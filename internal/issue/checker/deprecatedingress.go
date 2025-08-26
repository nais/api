package checker

import (
	"context"
	"strings"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

var deprecatedIngresses = map[string][]string{
	"dev-fss": {
		"adeo.no",
		"intern.dev.adeo.no",
		"dev-fss.nais.io",
		"dev.adeo.no",
		"dev.intern.nav.no",
		"nais.preprod.local",
	},
	"dev-gcp": {
		"dev-gcp.nais.io",
		"dev.intern.nav.no",
		"dev.nav.no",
		"intern.nav.no",
		"dev.adeo.no",
		"labs.nais.io",
		"ekstern.dev.nais.io",
	},
	"prod-fss": {
		"adeo.no",
		"nais.adeo.no",
		"prod-fss.nais.io",
	},
	"prod-gcp": {
		"dev.intern.nav.no",
		"prod-gcp.nais.io",
	},
}

type DeprecatedIngress struct {
	applicationLister KubernetesLister[*application.Application]
}

var _ Check = DeprecatedIngress{}

type DeprecatedIngressIssueDetails struct {
	Ingresses []string `json:"ingresses"`
}

func (d DeprecatedIngress) Run(ctx context.Context) ([]Issue, error) {
	envs, err := environment.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	ret := []Issue{}
	for _, env := range envs {
		apps := d.applicationLister.List(ctx, env.Name)
		for _, app := range apps {
			di := deprecated(app.Spec.Ingresses, env.Name)
			if len(di) > 0 {
				ret = append(ret, Issue{
					IssueType:    IssueTypeDeprecatedIngress,
					ResourceName: app.Name,
					ResourceType: "application",
					Team:         string(app.TeamSlug),
					Env:          env.Name,
					Severity:     SeverityTodo,
					IssueDetails: DeprecatedIngressIssueDetails{
						Ingresses: di,
					},
				})
			}
		}
	}
	return ret, nil
}

func deprecated(ingresses []nais_io_v1.Ingress, env string) []string {
	ret := []string{}
	for _, ingress := range ingresses {
		i := strings.Join(strings.Split(string(ingress), ".")[1:], ".")
		for _, deprecatedIngress := range deprecatedIngresses[env] {
			if strings.HasPrefix(i, deprecatedIngress) {
				ret = append(ret, string(ingress))
			}
		}
	}
	return ret
}

func (DeprecatedIngress) Supports(w workload.Workload) bool {
	_, ok := w.(*application.Application)
	return ok
}
