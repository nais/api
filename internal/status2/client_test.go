package status2_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/status2"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)

	}
	ctx := context.Background()
	token := os.Getenv("AIVEN_TOKEN")
	client, err := status2.NewClient(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	d, err := client.AivenStatus(ctx, "nav-prod", "opensearch-tbd-sparsom", "yolo")
	assert.NoError(t, err)
	assert.NotNil(t, d)
	spew.Dump(d)

}
