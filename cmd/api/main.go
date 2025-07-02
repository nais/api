package main

import (
	"context"
	"fmt"

	// Auto configure settings when running in a container
	_ "go.uber.org/automaxprocs"

	"github.com/nais/api/internal/cmd/api"
)

func main() {
	fmt.Println("Code change")
	ctx := context.Background()
	api.Run(ctx)
}
