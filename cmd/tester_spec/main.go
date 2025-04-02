package main

import (
	"context"
	"flag"
	"path/filepath"

	"github.com/nais/api/internal/integration"
)

func main() {
	dir := filepath.Join(".", "integration_tests")
	flag.StringVar(&dir, "d", dir, "write spec to this directory")
	flag.Parse()

	mgr, cleanup, err := integration.TestRunner(context.Background(), true)
	if err != nil {
		panic(err)
	}

	defer cleanup()

	if err := mgr.GenerateSpec(dir); err != nil {
		panic(err)
	}
}
