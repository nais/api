package issuechecker_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/nais/api/internal/issuechecker"
	"github.com/nais/api/internal/persistence/sqlinstance"
)

func TestRunChecks(t *testing.T) {
	ctx := context.Background()

	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}
	token := os.Getenv("AIVEN_TOKEN")

	i := issuechecker.New(issuechecker.Config{
		AivenToken:    token,
		AivenProjects: []string{"nav-prod", "nav-dev"},
	})

	i.SQLInstanceLister = &MockSQLInstanceLister{}
	i.RunChecks(ctx)
}

type MockSQLInstanceLister struct{}

func (m *MockSQLInstanceLister) List(ctx context.Context) []*sqlinstance.SQLInstance {
	return []*sqlinstance.SQLInstance{
		{
			Name:      "contests",
			TeamSlug:  "nais",
			ProjectID: "nais-prod-020f",
		},
	}
}
