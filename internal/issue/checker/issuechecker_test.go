package checker_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/nais/api/internal/issue/checker"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/stretchr/testify/assert"

	"github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
	"github.com/nais/api/internal/persistence/sqlinstance"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func TestRunChecks(t *testing.T) {
	ctx := context.Background()

	if err := godotenv.Load("../../../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}
	token := os.Getenv("AIVEN_TOKEN")
	connString := "postgres://api:api@127.0.0.1:3002/api?sslmode=disable"
	pool, err := database.NewPool(ctx, connString, logrus.New(), true)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	defer pool.Close()

	i := checker.New(checker.Config{
		AivenToken:    token,
		AivenProjects: []string{"nav-prod", "nav-dev"},
	}, pool)

	i.SQLInstanceLister = &MockSQLInstanceLister{}
	err = i.RunChecks(ctx)
	assert.NoError(t, err)
}

type MockApplicationLister struct{}

func (m *MockApplicationLister) List(ctx context.Context, env string) []*application.Application {
	return []*application.Application{
		{
			Base: workload.Base{
				Name:            "my-app",
				TeamSlug:        slug.Slug("nais"),
				EnvironmentName: "prod-gcp",
			},
			Spec: &nais_io_v1alpha1.ApplicationSpec{
				Ingresses: []nais_io_v1.Ingress{"test.dev.intern.nav.no"},
			},
		},
	}
}

type MockSQLInstanceLister struct{}

func (m *MockSQLInstanceLister) List(ctx context.Context, env string) []*sqlinstance.SQLInstance {
	return []*sqlinstance.SQLInstance{
		{
			Name:            "contests",
			TeamSlug:        "nais",
			ProjectID:       "nais-prod-020f",
			EnvironmentName: "prod-gcp",
		},
	}
}
