package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/nais/api/internal/v1/integration"
	"github.com/nais/tester/lua"
)

func main() {
	dir := filepath.Join(".", "integration_tests")
	ui := false
	flag.BoolVar(&ui, "ui", ui, "run with UI")
	flag.StringVar(&dir, "d", dir, "write spec to this directory")
	flag.Parse()

	ctx := context.Background()
	mgr, err := integration.TestRunner(ctx, false)
	if err != nil {
		panic(err)
	}

	if err := mgr.GenerateSpec(dir); err != nil {
		panic(err)
	}

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
