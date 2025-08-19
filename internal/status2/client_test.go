package status2_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/status2"
	"github.com/stretchr/testify/assert"
)

type FakeInfo struct{}

func (f FakeInfo) OpenSearches(ctx context.Context) ([]*opensearch.OpenSearch, error) {
	return []*opensearch.OpenSearch{
		{
			Name:            "opensearch-tbd-sparsom",
			AivenProject:    "nav-prod",
			TeamSlug:        "tbd",
			EnvironmentName: "prod-gcp",
		},
		{
			Name:            "opensearch-tbd-sparsom",
			AivenProject:    "nav-dev",
			TeamSlug:        "tbd",
			EnvironmentName: "dev-gcp",
		},
	}, nil
}

func (f FakeInfo) SQLInstances(ctx context.Context) ([]sqlinstance.SQLInstance, error) {
	return []sqlinstance.SQLInstance{
		{
			Name:            "mulighetsrommet-api-v1",
			ProjectID:       "team-mulighetsrommet-dev-a2d7",
			TeamSlug:        "team-mulighetsrommet",
			EnvironmentName: "dev-gcp",
		},
		{
			Name:            "spinnvill-clone-for-fremtidig-feilretting",
			ProjectID:       "tbd-prod-eacd",
			TeamSlug:        "tbd",
			EnvironmentName: "prod-gcp",
		},
	}, nil
}

func TestNewClient(t *testing.T) {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)

	}
	ctx := context.Background()
	token := os.Getenv("AIVEN_TOKEN")
	client, err := status2.NewClient(ctx, token, FakeInfo{})
	assert.NoError(t, err)
	assert.NotNil(t, client)

	statuses, err := client.Run(ctx)
	assert.NoError(t, err)

	spew.Dump(statuses)
}
