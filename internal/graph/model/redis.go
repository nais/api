package model

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/api/internal/graph/scalar"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Redis struct {
	Name    string       `json:"name"`
	Access  string       `json:"access"`
	ID      scalar.Ident `json:"id"`
	Env     Env          `json:"env"`
	GQLVars RedisGQLVars `json:"-"`
}

type RedisGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (Redis) IsPersistence()    {}
func (r Redis) GetName() string { return r.Name }

func ToRedis(u *unstructured.Unstructured, env string) (*Redis, error) {
	redis := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, redis); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	projectId := redis.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	teamSlug := redis.GetNamespace()

	return &Redis{
		ID: scalar.RedisIdent("redis_" + env + "_" + teamSlug + "_" + redis.GetName()),

		Name: redis.Name,
		Env: Env{
			Name: env,
			Team: redis.GetNamespace(),
		},
		GQLVars: RedisGQLVars{
			TeamSlug:       slug.Slug(redis.GetNamespace()),
			OwnerReference: OwnerReference(redis.OwnerReferences),
		},
		// ProjectID: projectId,
	}, nil
}