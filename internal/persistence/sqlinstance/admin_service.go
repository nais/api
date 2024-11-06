package sqlinstance

import (
	"context"
	"sort"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
)

type SQLAdminService struct {
	cache  *cache.Cache
	client *sqladmin.Service
	log    logrus.FieldLogger
}

func NewSQLAdminService(ctx context.Context, log logrus.FieldLogger, opts ...option.ClientOption) (*SQLAdminService, error) {
	sqladminService, err := sqladmin.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &SQLAdminService{
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		client: sqladminService,
		log:    log,
	}, nil
}

func (s *SQLAdminService) GetUsers(ctx context.Context, project string, instance string) ([]*sqladmin.User, error) {
	key := "users:" + project + ":" + instance
	if users, found := s.cache.Get(key); found {
		return users.([]*sqladmin.User), nil
	}

	users, err := s.client.Users.List(project, instance).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	sort.SliceStable(users.Items, func(i, j int) bool {
		if users.Items[i].Type == users.Items[j].Type {
			return users.Items[i].Name < users.Items[j].Name
		}
		return users.Items[i].Type < users.Items[j].Type
	})

	s.cache.Set(key, users.Items, cache.DefaultExpiration)
	return users.Items, nil
}

func (s *SQLAdminService) GetInstance(ctx context.Context, project string, instance string) (*sqladmin.DatabaseInstance, error) {
	key := "instance:" + project + ":" + instance
	if i, found := s.cache.Get(key); found {
		return i.(*sqladmin.DatabaseInstance), nil
	}

	i, err := s.client.Instances.Get(project, instance).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	s.cache.Set(key, i, cache.DefaultExpiration)
	return i, nil
}
