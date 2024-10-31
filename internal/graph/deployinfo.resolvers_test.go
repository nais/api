package graph_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_deployInfoResolver_History(t *testing.T) {
	ctx := context.Background()

	deployInfo := &model.DeployInfo{
		GQLVars: model.DeployInfoGQLVars{
			App:  "some-app-name",
			Job:  "job",
			Env:  "production",
			Team: "some-team",
		},
	}
	hookdClient := hookd.NewMockClient(t)
	hookdClient.
		EXPECT().
		Deployments(ctx, mock.AnythingOfType("hookd.RequestOption"), mock.AnythingOfType("hookd.RequestOption")).
		Run(func(_ context.Context, opts ...hookd.RequestOption) {
			assert.Len(t, opts, 2)
			r, _ := http.NewRequest("GET", "http://example.com", nil)
			for _, opt := range opts {
				opt(r)
			}
			assert.Contains(t, r.URL.RawQuery, "team=some-team")
			assert.Contains(t, r.URL.RawQuery, "cluster=production")
		}).
		Return([]hookd.Deploy{
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-1"},
				Resources: []hookd.Resource{
					{ID: "resource-id-1", Name: "job", Kind: "Application"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-2"},
				Resources: []hookd.Resource{
					{ID: "resource-id-2", Name: "job", Kind: "Job"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-3"},
				Resources: []hookd.Resource{
					{ID: "resource-id-3", Name: "job", Kind: "Naisjob"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-4"},
				Resources: []hookd.Resource{
					{ID: "resource-id-4", Name: "job", Kind: "Naisjob"},
					{ID: "resource-id-5", Name: "job", Kind: "Job"},
					{ID: "resource-id-6", Name: "job", Kind: "Application"},
					// Last entry has same name/kind as first entry on purpose. Not sure if this occurs in the wild, but
					// we should handle it gracefully.
					{ID: "resource-id-7", Name: "job", Kind: "Naisjob"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-5"},
				Resources: []hookd.Resource{
					{ID: "resource-id-8", Name: "foo", Kind: "Application"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-6"},
				Resources: []hookd.Resource{
					{ID: "resource-id-9", Name: "foo", Kind: "Job"},
				},
			},
			{
				DeploymentInfo: hookd.DeploymentInfo{ID: "id-7"},
				Resources: []hookd.Resource{
					{ID: "resource-id-10", Name: "foo", Kind: "Naisjob"},
				},
			},
		}, nil)

	resp, err := graph.
		NewResolver(
			hookdClient,
			nil,
			nil,
			nil,
			nil,
			"example",
			"example.com",
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		).
		DeployInfo().
		History(ctx, deployInfo, nil, nil)
	assert.NoError(t, err)

	conn, ok := resp.(*model.DeploymentList)
	assert.True(t, ok)
	assert.Len(t, conn.Nodes, 2)

	assert.Equal(t, "id-3", conn.Nodes[0].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeployment, conn.Nodes[0].ID.Type)
	assert.Len(t, conn.Nodes[0].Resources, 1)
	assert.Equal(t, "resource-id-3", conn.Nodes[0].Resources[0].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeploymentResource, conn.Nodes[0].Resources[0].ID.Type)

	assert.Equal(t, "id-4", conn.Nodes[1].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeployment, conn.Nodes[1].ID.Type)
	assert.Len(t, conn.Nodes[1].Resources, 4)
	assert.Equal(t, "resource-id-4", conn.Nodes[1].Resources[0].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeploymentResource, conn.Nodes[1].Resources[0].ID.Type)
	assert.Equal(t, "resource-id-5", conn.Nodes[1].Resources[1].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeploymentResource, conn.Nodes[1].Resources[1].ID.Type)
	assert.Equal(t, "resource-id-6", conn.Nodes[1].Resources[2].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeploymentResource, conn.Nodes[1].Resources[2].ID.Type)
	assert.Equal(t, "resource-id-7", conn.Nodes[1].Resources[3].ID.ID)
	assert.Equal(t, scalar.IdentTypeDeploymentResource, conn.Nodes[1].Resources[3].ID.Type)
}
