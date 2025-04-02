//go:build integration_test

package integrationtests

import (
	"context"
	"testing"

	"github.com/nais/api/internal/integration"
	"github.com/nais/tester/lua"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	mgr, cleanup, err := integration.TestRunner(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	defer cleanup()

	if err := mgr.Run(ctx, ".", lua.NewTestReporter(t)); err != nil {
		t.Fatal(err)
	}
}
