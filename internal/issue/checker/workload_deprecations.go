package checker

import (
	"fmt"
	"strings"

	"github.com/nais/api/internal/issue"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func deprecatedRegistry(image, name, team, env string, resourceType issue.ResourceType) *Issue {
	allowedRegistries := []string{
		"europe-north1-docker.pkg.dev",
		"repo.adeo.no:5443",
		"oliver006/redis_exporter",
		"bitnami/redis",
		"docker.io/oliver006/redis_exporter",
		"docker.io/redis",
		"docker.io/bitnami/redis",
		"redis",
		"krakend",
	}

	for _, registry := range allowedRegistries {
		if strings.HasPrefix(image, registry) {
			return nil
		}
	}

	return &Issue{
		IssueType:    issue.IssueTypeDeprecatedRegistry,
		ResourceName: name,
		ResourceType: resourceType,
		Team:         team,
		Env:          env,
		Severity:     issue.SeverityWarning,
		Message:      fmt.Sprintf("Image '%s' is using a deprecated registry", image),
	}
}

func deprecatedIngress(app *nais_io_v1alpha1.Application, env string) *Issue {
	deprecatedIngressList := map[string][]string{
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

	deprecated := func(ingresses []nais_io_v1.Ingress, env string) []string {
		ret := make([]string, 0)
		for _, ingress := range ingresses {
			i := strings.Join(strings.Split(string(ingress), ".")[1:], ".")
			for _, deprecatedIngress := range deprecatedIngressList[env] {
				if strings.HasPrefix(i, deprecatedIngress) {
					ret = append(ret, string(ingress))
				}
			}
		}
		return ret
	}

	di := deprecated(app.Spec.Ingresses, env)

	if len(di) == 0 {
		return nil
	}

	return &Issue{
		IssueType:    issue.IssueTypeDeprecatedIngress,
		ResourceName: app.Name,
		ResourceType: issue.ResourceTypeApplication,
		Team:         app.GetNamespace(),
		Env:          env,
		Severity:     issue.SeverityTodo,
		Message:      fmt.Sprintf("Application is using deprecated ingresses: %v", di),
		IssueDetails: issue.DeprecatedIngressIssueDetails{
			Ingresses: di,
		},
	}
}
