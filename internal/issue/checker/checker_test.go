package checker_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue/checker"
	"github.com/stretchr/testify/assert"

	"github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
	"github.com/nais/api/internal/persistence/sqlinstance"
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

	environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{
		"prod": "prod-gcp",
		"dev":  "dev-gcp",
	})

	defer pool.Close()

	i, err := checker.New(ctx, checker.Config{
		AivenToken: token,
		Tenant:     "nav",
	},
		pool,
		checker.WithSQLInstanceLister(&MockSQLInstanceLister{}),
		checker.WithApplicationLister(&MockApplicationLister{}),
	)

	if err != nil {
		log.Fatalf("failed to create checker: %v", err)
	}

	err = i.RunChecks(ctx)
	assert.NoError(t, err)
}

type MockSQLInstanceLister struct{}

func (m *MockSQLInstanceLister) List(ctx context.Context) []*sqlinstance.SQLInstance {
	return []*sqlinstance.SQLInstance{
		{
			Name:            "contests",
			TeamSlug:        "nais",
			ProjectID:       "nais-prod-020f",
			EnvironmentName: "prod-gcp",
		},
	}
}
