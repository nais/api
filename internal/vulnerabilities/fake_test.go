//go:build integration
// +build integration

package vulnerabilities

import (
	"encoding/json"
	"fmt"
	"github.com/nais/dependencytrack/pkg/client"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

func TestMaskFakeData(t *testing.T) {

	// replace file with file containing real data
	b, err := os.ReadFile("fakedata/projects.json")
	assert.NoError(t, err)

	var projects []*client.Project
	err = json.Unmarshal(b, &projects)
	assert.NoError(t, err)
	filtered := make([]*client.Project, 0)
	for i, p := range projects {
		// skip wonderwall images
		if strings.Contains(p.Name, "wonderwall") {
			continue
		}

		if i > 8 {
			break
		}

		team := "devteam"
		version := p.Version
		workloadName := fmt.Sprintf("nais-deploy-chicken-%d", i+1)
		workloadType := "app"
		env := "dev"
		projectName := fmt.Sprintf("europe-north1-docker.pkg.dev/nais-management-233d/%s/%s", team, workloadName)

		if i%2 == 0 {
			workloadType = "job"
		}

		p.Name = projectName
		tags := createTags(env, team, workloadType, workloadName, version)
		tags = append(tags, createTags("superprod", team, workloadType, workloadName, version)...)
		p.Tags = tags

		filtered = append(filtered, p)
	}

	out, err := json.MarshalIndent(filtered, "", "  ")
	assert.NoError(t, err)
	err = os.WriteFile("fakedata/projects.json", out, 0644)
	assert.NoError(t, err)
}

func createTags(env, team, workloadType, workloadName, version string) []client.Tag {
	tags := make([]client.Tag, 0)

	tags = append(tags, createTag(fmt.Sprintf("version:%s", version)))
	tags = append(tags, createTag(fmt.Sprintf("image:europe-north1-docker.pkg.dev/nais-management-233d/%s/%s:%s", team, workloadName, version)))
	tags = append(tags, createTag(fmt.Sprintf("project:europe-north1-docker.pkg.dev/nais-management-233d/%s/%s", team, workloadName)))
	tags = append(tags, createTag(fmt.Sprintf("env:%s", env)))
	tags = append(tags, createTag(fmt.Sprintf("team:%s", team)))
	tags = append(tags, createTag(fmt.Sprintf("workload:%s|%s|%s|%s", env, team, workloadType, workloadName)))

	return tags
}

func createTag(name string) client.Tag {
	return client.Tag{Name: name}
}
