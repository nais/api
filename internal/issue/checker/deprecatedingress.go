package checker

import (
	"context"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
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
}

var _ check = DeprecatedIngress{}

func (d DeprecatedIngress) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)
	apps := d.ApplicationLister.List(ctx)
	for _, app := range apps {
		env := environmentmapper.EnvironmentName(app.Cluster)
		di := deprecated(app.Obj.Spec.Ingresses, env)
		if len(di) > 0 {
			ret = append(ret, Issue{
				IssueType:    issue.IssueTypeDeprecatedIngress,
				ResourceName: app.Obj.Name,
				ResourceType: issue.ResourceTypeApplication,
				Team:         app.GetNamespace(),
				Env:          env,
				Severity:     issue.SeverityTodo,
				Message:      "Deprecated ingress detected",
				IssueDetails: issue.DeprecatedIngressIssueDetails{
					Ingresses: di,
				},
			})
		}
	}
	return ret, nil
}

func deprecated(ingresses []nais_io_v1.Ingress, env string) []string {
	ret := make([]string, 0)
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
