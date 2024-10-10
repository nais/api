package integrationtests

import (
	"context"
	"testing"

	"github.com/nais/api/internal/v1/integration"
	"github.com/nais/tester/lua"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	mgr, err := integration.TestRunner(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Run(ctx, ".", lua.NewTestReporter(t)); err != nil {
		t.Fatal(err)
	}
}
