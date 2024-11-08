package fake

import (
	"context"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

type knownKeys struct {
	lock sync.RWMutex
	mp   map[string]*hookd.DeployKey
}

func (k *knownKeys) Get(team string) *hookd.DeployKey {
	k.lock.RLock()
	defer k.lock.RUnlock()
	return k.mp[team]
}

func (k *knownKeys) Set(team string, key *hookd.DeployKey) {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.mp[team] = key
}

func (k *knownKeys) Has(team string) bool {
	k.lock.RLock()
	defer k.lock.RUnlock()
	_, ok := k.mp[team]
	return ok
}

type FakeHookdClient struct {
	keys knownKeys
}

func New() *FakeHookdClient {
	return &FakeHookdClient{
		keys: knownKeys{
			mp: make(map[string]*hookd.DeployKey),
		},
	}
}

func (f *FakeHookdClient) ChangeDeployKey(ctx context.Context, team string) (*hookd.DeployKey, error) {
	n := &hookd.DeployKey{
		Team:    team,
		Key:     uuid.New().String(),
		Expires: time.Now().Add(365 * 24 * time.Hour),
		Created: time.Now(),
	}

	f.keys.Set(team, n)
	return n, nil
}

func (f *FakeHookdClient) DeployKey(ctx context.Context, team string) (*hookd.DeployKey, error) {
	if !f.keys.Has(team) {
		return f.ChangeDeployKey(ctx, team)
	}
	return f.keys.Get(team), nil
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
	for i := 0; i < rand.IntN(30); i++ {
		ret = append(ret, newDeploy(cluster, slug.Slug(team)))
	}

	return ret, nil
}

func newDeploy(cluster string, team slug.Slug) hookd.Deploy {
	num := rand.IntN(3) + 1
	parts := make([]string, num)
	for i := 0; i < num; i++ {
		parts[i] = nameParts[rand.IntN(len(nameParts))]
	}

	name := "fake-" + strings.Join(parts, "-")

	deploy := hookd.Deploy{
		DeploymentInfo: hookd.DeploymentInfo{
			ID:               uuid.New().String(),
			Team:             team,
			Cluster:          cluster,
			Created:          time.Now().Add(-time.Duration(rand.IntN(1000)) * time.Hour),
			GithubRepository: "somerepo",
		},
	}

	deploy.Statuses = append(deploy.Statuses, hookd.Status{
		ID:      uuid.New().String(),
		Status:  statuses[rand.IntN(len(statuses))],
		Message: "Some message",
		Created: time.Now().Add(-time.Duration(rand.IntN(1000)) * time.Hour),
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
