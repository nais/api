package status

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
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

type checkDeprecatedIngress struct{}

func (checkDeprecatedIngress) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	app := w.(*application.Application)
	var ret []WorkloadStatusError
	for _, ingress := range app.Ingresses() {
		i := strings.Join(strings.Split(ingress, ".")[1:], ".")
		for _, deprecatedIngress := range deprecatedIngresses[app.EnvironmentName] {
			if i == deprecatedIngress {
				ret = append(ret, &WorkloadStatusDeprecatedIngress{
					Level:   WorkloadStatusErrorLevelTodo,
					Ingress: ingress,
				})
			}
		}
	}

	return ret, WorkloadStateNais
}

func (checkDeprecatedIngress) Supports(w workload.Workload) bool {
	_, ok := w.(*application.Application)
	return ok
}
