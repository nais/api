package fake

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/thirdparty/hookd"
)

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

func (f *FakeHookdClient) ChangeDeployKey(_ context.Context, team string) (*hookd.DeployKey, error) {
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
