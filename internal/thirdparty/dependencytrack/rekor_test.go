package dependencytrack

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogEntryToPubKey(t *testing.T) {
	entry, err := os.ReadFile("testdata/dsse-envelope.json")
	assert.NoError(t, err)
	pubKey, err := logEntryToPubKey(entry)
	assert.NoError(t, err)
	assert.NotEmpty(t, pubKey)
}

func TestCertToRekorMetadata(t *testing.T) {
	cert, err := os.ReadFile("testdata/rekor-verifier.txt")
	assert.NoError(t, err)
	data, err := certToRekorMetadata(cert, int64(1629780000))
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Equalf(t, "Build and deploy picante", data.GitHubWorkflowName, "data.GitHubWorkflowName")
	assert.Equalf(t, "push", data.BuildTrigger, "data.BuildTrigger")
	assert.Equalf(t, "refs/heads/main", data.GitHubWorkflowRef, "data.GitHubWorkflowRef")
	assert.Equalf(t, "https://token.actions.githubusercontent.com", data.OIDCIssuer, "data.OIDCIssuer")
	assert.Equalf(t, "https://github.com/nais/picante/actions/runs/5080120886/attempts/1", data.RunInvocationURI, "data.RunInvocationURI")
	assert.Equalf(t, "github-hosted", data.RunnerEnvironment, "data.RunnerEnvironment")
	assert.Equalf(t, "https://github.com/nais", data.SourceRepositoryOwnerURI, "data.SourceRepositoryOwnerURI")
	assert.Equalf(t, "https://github.com/nais/picante/.github/workflows/build.yml@refs/heads/main", data.BuildConfigURI, "data.BuildConfigURI")
	assert.Equalf(t, 1629780000, data.IntegratedTime, "data.IntegratedTime")
}
