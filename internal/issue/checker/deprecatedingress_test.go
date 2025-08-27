package checker_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/issue/checker"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func TestDeprecatedIngress(t *testing.T) {
	check := checker.DeprecatedIngress{ApplicationLister: MockApplicationLister{}}
	issues, err := check.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}
}

type MockApplicationLister struct{}

func (m MockApplicationLister) List(ctx context.Context) []*application.Application {
	return []*application.Application{
		{
			Base: workload.Base{
				Name:            "my-app",
				TeamSlug:        slug.Slug("tbd"),
				EnvironmentName: "prod-gcp",
			},
			Spec: &nais_io_v1alpha1.ApplicationSpec{
				Ingresses: []nais_io_v1.Ingress{"test.dev.intern.nav.no"},
			},
		},
	}
}
