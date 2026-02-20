package main

import (
	"context"

	genragindex "github.com/nais/api/internal/cmd/gen_rag_index"
)

func main() {
	ctx := context.Background()
	genragindex.Run(ctx)
}
