package sqlinstance

import (
	"context"
	"errors"
	"sort"
	"time"

	"google.golang.org/api/option"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sqladmin/v1"
)

type SqlAdminService struct {
	cache  *cache.Cache
	client *sqladmin.Service
	log    logrus.FieldLogger
}

func NewSqlAdminService(ctx context.Context, log logrus.FieldLogger, opts ...option.ClientOption) (*SqlAdminService, error) {
	sqladminService, err := sqladmin.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &SqlAdminService{
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		client: sqladminService,
		log:    log,
	}, nil
}

func (s *SqlAdminService) GetState(ctx context.Context, project string, instance string) (string, error) {
	i, err := s.GetInstance(ctx, project, instance)
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) {
			if googleErr.Code == 404 {
				s.log.WithError(err).Info("could not get instance, instance not found")
				return "SQL_INSTANCE_STATE_UNSPECIFIED", nil
			}
		}
		return "", err
	}
	return i.State, nil
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

	sort.SliceStable(users.Items, func(i, j int) bool {
		if users.Items[i].Type == users.Items[j].Type {
			return users.Items[i].Name < users.Items[j].Name
		}
		return users.Items[i].Type < users.Items[j].Type
	})

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

func (s *SqlAdminService) GetInstance(ctx context.Context, project string, instance string) (*sqladmin.DatabaseInstance, error) {
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
