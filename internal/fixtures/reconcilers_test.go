package fixtures_test

import (
	"fmt"
	"testing"

	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/sqlc"
	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		input           string
		expectedOutput  fixtures.EnableableReconciler
		expectedErrText string
	}{
		{
			input:           "invalid",
			expectedErrText: "reconciler \"invalid\" cannot be enabled on first run",
		},
		{
			input:          string(sqlc.ReconcilerNameNaisNamespace),
			expectedOutput: fixtures.EnableableReconciler(sqlc.ReconcilerNameNaisNamespace),
		},
		{
			input:          string(sqlc.ReconcilerNameNaisDeploy),
			expectedOutput: fixtures.EnableableReconciler(sqlc.ReconcilerNameNaisDeploy),
		},
		{
			input:          string(sqlc.ReconcilerNameGoogleGcpProject),
			expectedOutput: fixtures.EnableableReconciler(sqlc.ReconcilerNameGoogleGcpProject),
		},
		{
			input:          string(sqlc.ReconcilerNameGoogleWorkspaceAdmin),
			expectedOutput: fixtures.EnableableReconciler(sqlc.ReconcilerNameGoogleWorkspaceAdmin),
		},
		{
			input:           string(sqlc.ReconcilerNameGithubTeam),
			expectedErrText: fmt.Sprintf("reconciler %q cannot be enabled on first run", sqlc.ReconcilerNameGithubTeam),
		},
		{
			input:           string(sqlc.ReconcilerNameAzureGroup),
			expectedErrText: fmt.Sprintf("reconciler %q cannot be enabled on first run", sqlc.ReconcilerNameAzureGroup),
		},
	}
	for _, tt := range tests {
		var decoded fixtures.EnableableReconciler = ""
		err := decoded.Decode(tt.input)

		if tt.expectedErrText != "" {
			assert.ErrorContains(t, err, tt.expectedErrText)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, decoded)
		}
	}
}
