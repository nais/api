package sqlinstance

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/sqladmin/v1"
)

type SqlAdminService struct {
	cache  *cache.Cache
	client *sqladmin.Service
	log    logrus.FieldLogger
}

func NewSqlAdminService(ctx context.Context, log logrus.FieldLogger) (*SqlAdminService, error) {
	sqladminService, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, err
	}

	return &SqlAdminService{
		cache:  cache.New(60*time.Minute, 70*time.Minute),
		client: sqladminService,
		log:    log,
	}, nil
}

func (s *SqlAdminService) GetUsers(ctx context.Context, project string, instance string) ([]*sqladmin.User, error) {
	key := "users:" + project + ":" + instance
	if users, found := s.cache.Get(key); found {
		return users.([]*sqladmin.User), nil
	}

	users, err := s.client.Users.List(project, instance).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	s.cache.Set(key, users.Items, cache.DefaultExpiration)
	return users.Items, nil
}

func (s *SqlAdminService) GetInstances(ctx context.Context, project string) ([]*sqladmin.DatabaseInstance, error) {
	key := "instances:" + project
	if instances, found := s.cache.Get(key); found {
		return instances.([]*sqladmin.DatabaseInstance), nil
	}

	instances, err := s.client.Instances.List(project).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	s.cache.Set(key, instances.Items, cache.DefaultExpiration)
	return instances.Items, nil
}
