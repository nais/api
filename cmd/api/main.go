package main

import (
	"context"

	"github.com/nais/api/internal/cmd/api"
)

func main() {
	ctx := context.Background()
	api.Run(ctx)
}
