package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/nais/api/internal/integration"
	"github.com/nais/tester/lua"
)

func main() {
	dir := filepath.Join(".", "integration_tests")
	ui := false
	flag.BoolVar(&ui, "ui", ui, "run with UI")
	flag.Parse()

	ctx := context.Background()
	mgr, cleanup, err := integration.TestRunner(ctx, false)
	if err != nil {
		panic(err)
	}

	defer cleanup()

	if ui {
		if err := mgr.RunUI(ctx, dir); err != nil {
			panic(err)
		}
	} else {
		if err := mgr.Run(ctx, dir, lua.NewJSONReporter(os.Stdout)); err != nil {
			panic(err)
		}
	}
}
