package issuechecker_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/nais/api/internal/issuechecker"
)

func TestRunChecks(t *testing.T) {
	ctx := context.Background()

	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)

	}
	token := os.Getenv("AIVEN_TOKEN")

	issuechecker.New(issuechecker.Config{
		AivenToken:    token,
		AivenProjects: []string{"nav-prod", "nav-dev"},
	}).RunChecks(ctx)

}
