package aivencache

import (
	"context"
	"fmt"
	"sync"
	"time"

	aiven "github.com/aiven/go-client-codegen"
	"github.com/aiven/go-client-codegen/handler/project"
	"github.com/aiven/go-client-codegen/handler/service"
	"github.com/patrickmn/go-cache"
)

type client struct {
	cache       *cache.Cache
	aivenClient aiven.Client
	lock        *sync.Mutex
}

func NewClient(aivenClient aiven.Client) AivenClient {
	return &client{
		cache:       cache.New(time.Minute, 2*time.Minute),
		aivenClient: aivenClient,
		lock:        &sync.Mutex{},
	}
}

func (c *client) ServiceGet(ctx context.Context, projectName string, serviceName string, query ...[2]string) (*service.ServiceGetOut, error) {
	key := makeKey(projectName, serviceName)
	if cached, found := c.cache.Get(key); found {
		if serviceGetOut, ok := cached.(*service.ServiceGetOut); ok {
			return serviceGetOut, nil
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// Check if it has arrived in the cache while we were waiting
	if cached, found := c.cache.Get(key); found {
		if serviceGetOut, ok := cached.(*service.ServiceGetOut); ok {
			return serviceGetOut, nil
		}
	}

	serviceGetOut, err := c.aivenClient.ServiceGet(ctx, projectName, serviceName, query...)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, serviceGetOut, cache.DefaultExpiration)
	return serviceGetOut, nil
}

func (c *client) ServiceMaintenanceStart(ctx context.Context, projectName string, serviceName string) error {
	key := makeKey(projectName, serviceName)
	c.cache.Delete(key)
	return c.aivenClient.ServiceMaintenanceStart(ctx, projectName, serviceName)
}

func (c *client) ProjectAlertsList(ctx context.Context, project string) ([]project.AlertOut, error) {
	return c.aivenClient.ProjectAlertsList(ctx, project)
}

func makeKey(projectName, serviceName string) string {
	return fmt.Sprintf("%s/%s", projectName, serviceName)
}
