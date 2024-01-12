package fake

import (
	"context"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
)

var nameParts = []string{
	"nais",
	"api",
	"test",
	"q0",
	"q1",
	"bidrag",
	"up",
	"down",
	"checker",
	"service",
	"operator",
}

var statuses = []string{
	"success",
	"in_progress",
	"queued",
	"failure",
}

type FakeHookdClient struct{}

func New() *FakeHookdClient {
	return &FakeHookdClient{}
}

func (f *FakeHookdClient) ChangeDeployKey(ctx context.Context, team string) (*hookd.DeployKey, error) {
	return &hookd.DeployKey{
		Team:    team,
		Key:     uuid.New().String(),
		Expires: time.Now().Add(365 * 24 * time.Hour),
		Created: time.Now(),
	}, nil
}

func (f *FakeHookdClient) DeployKey(ctx context.Context, team string) (*hookd.DeployKey, error) {
	return &hookd.DeployKey{
		Team:    team,
		Key:     uuid.New().String(),
		Expires: time.Now().Add(365 * 24 * time.Hour),
		Created: time.Now(),
	}, nil
}

func (f *FakeHookdClient) Deployments(ctx context.Context, opts ...hookd.RequestOption) ([]hookd.Deploy, error) {
	team := "nais"
	cluster := "dev"

	if len(opts) > 0 {
		r := &http.Request{URL: &url.URL{}}
		for _, opt := range opts {
			opt(r)
		}

		if t := r.URL.Query().Get("team"); t != "" {
			team = t
		}

		if c := r.URL.Query().Get("cluster"); c != "" {
			cluster = c
		}
	}

	ret := []hookd.Deploy{}
	for i := 0; i < rand.Intn(30); i++ {
		ret = append(ret, newDeploy(cluster, slug.Slug(team)))
	}

	return ret, nil
}

func newDeploy(cluster string, team slug.Slug) hookd.Deploy {
	num := rand.Intn(3) + 1
	parts := make([]string, num)
	for i := 0; i < num; i++ {
		parts[i] = nameParts[rand.Intn(len(nameParts))]
	}

	name := "fake-" + strings.Join(parts, "-")

	deploy := hookd.Deploy{
		DeploymentInfo: hookd.DeploymentInfo{
			ID:               name,
			Team:             team,
			Cluster:          cluster,
			Created:          time.Now().Add(-time.Duration(rand.Intn(1000)) * time.Hour),
			GithubRepository: "somerepo",
		},
	}

	deploy.Statuses = append(deploy.Statuses, hookd.Status{
		ID:      uuid.New().String(),
		Status:  statuses[rand.Intn(len(statuses))],
		Message: "Some message",
		Created: time.Now().Add(-time.Duration(rand.Intn(1000)) * time.Hour),
	})

	deploy.Resources = append(deploy.Resources, hookd.Resource{
		Kind:      "Application",
		Group:     "nais.io",
		Version:   "v1alpha1",
		ID:        uuid.New().String(),
		Name:      name,
		Namespace: team.String(),
	})

	return deploy
}
