package checker

import (
	"context"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"strings"

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
	ApplicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
	Environments      []string
}

var _ Check = DeprecatedIngress{}

type DeprecatedIngressIssueDetails struct {
	Ingresses []string `json:"ingresses"`
}

func (d DeprecatedIngress) Run(ctx context.Context) ([]Issue, error) {
	ret := []Issue{}
	apps := d.ApplicationLister.List(ctx)
	for _, app := range apps {
		env := environmentmapper.EnvironmentName(app.Cluster)
		di := deprecated(app.Obj.Spec.Ingresses, env)
		if len(di) > 0 {
			ret = append(ret, Issue{
				IssueType:    IssueTypeDeprecatedIngress,
				ResourceName: app.Obj.Name,
				ResourceType: "application",
				Team:         app.GetNamespace(),
				Env:          env,
				Severity:     SeverityTodo,
				IssueDetails: DeprecatedIngressIssueDetails{
					Ingresses: di,
				},
			})
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
